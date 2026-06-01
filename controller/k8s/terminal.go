/*
Copyright 2021 The DnsJia Authors.
WebSite:  https://github.com/dnsjia/luban
Email:    OpenSource@dnsjia.com

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package k8s

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/dnsjia/luban/common"
	"github.com/dnsjia/luban/pkg/k8s/Init"
	"github.com/dnsjia/luban/services"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

var k8sTerminalUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type terminalWSMessage struct {
	Op   string `json:"op"`
	Data string `json:"data"`
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
}

type websocketTerminal struct {
	conn     *websocket.Conn
	writeMu  sync.Mutex
	closeMu  sync.Mutex
	closed   bool
	readBuf  []byte
	sizeChan chan remotecommand.TerminalSize
	doneChan chan struct{}
}

func newWebsocketTerminal(conn *websocket.Conn) *websocketTerminal {
	return &websocketTerminal{
		conn:     conn,
		sizeChan: make(chan remotecommand.TerminalSize, 4),
		doneChan: make(chan struct{}),
	}
}

func (t *websocketTerminal) Read(p []byte) (int, error) {
	for len(t.readBuf) == 0 {
		_, payload, err := t.conn.ReadMessage()
		if err != nil {
			return copy(p, "\x04"), err
		}

		var msg terminalWSMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			t.readBuf = payload
			break
		}

		switch msg.Op {
		case "stdin":
			t.readBuf = []byte(msg.Data)
		case "resize":
			select {
			case t.sizeChan <- remotecommand.TerminalSize{Width: msg.Cols, Height: msg.Rows}:
			default:
			}
		default:
			return copy(p, "\x04"), fmt.Errorf("unknown terminal message op %q", msg.Op)
		}
	}

	n := copy(p, t.readBuf)
	t.readBuf = t.readBuf[n:]
	return n, nil
}

func (t *websocketTerminal) Write(p []byte) (int, error) {
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	if err := t.conn.WriteMessage(websocket.TextMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (t *websocketTerminal) Next() *remotecommand.TerminalSize {
	select {
	case size := <-t.sizeChan:
		return &size
	case <-t.doneChan:
		return nil
	}
}

func (t *websocketTerminal) close() {
	t.closeMu.Lock()
	defer t.closeMu.Unlock()
	if t.closed {
		return
	}
	t.closed = true
	close(t.doneChan)
	_ = t.conn.Close()
}

func (t *websocketTerminal) writeLine(format string, args ...interface{}) {
	_, _ = t.Write([]byte(fmt.Sprintf(format, args...) + "\r\n"))
}

func K8sTerminalController(c *gin.Context) {
	conn, err := k8sTerminalUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		common.LOG.Error(fmt.Sprintf("upgrade k8s terminal websocket failed: %v", err))
		return
	}

	terminal := newWebsocketTerminal(conn)
	defer terminal.close()

	client, restConfig, err := terminalK8sClient(c)
	if err != nil {
		terminal.writeLine("连接集群失败: %v", err)
		return
	}

	namespace := c.Query("namespace")
	podName := c.Query("pod")
	if namespace == "" || podName == "" {
		terminal.writeLine("缺少 namespace 或 pod 参数")
		return
	}

	containerName, err := terminalContainerName(c, client, namespace, podName)
	if err != nil {
		terminal.writeLine("获取容器失败: %v", err)
		return
	}

	terminal.writeLine("已连接 Pod %s/%s 容器 %s", namespace, podName, containerName)
	if err := streamPodShell(c, client, restConfig, terminal, namespace, podName, containerName); err != nil {
		terminal.writeLine("终端已断开: %v", err)
		return
	}
	terminal.writeLine("终端已退出")
}

func terminalK8sClient(c *gin.Context) (*kubernetes.Clientset, *rest.Config, error) {
	clusterID := c.DefaultQuery("clusterId", "1")
	clusterIDUint, err := strconv.ParseUint(clusterID, 10, 32)
	if err != nil {
		return nil, nil, err
	}
	cluster, err := services.GetK8sCluster(uint(clusterIDUint))
	if err != nil {
		return nil, nil, err
	}
	restConfig, err := Init.GetRestConf(cluster.KubeConfig)
	if err != nil {
		return nil, nil, err
	}
	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, nil, err
	}
	return client, restConfig, nil
}

func terminalContainerName(c *gin.Context, client *kubernetes.Clientset, namespace, podName string) (string, error) {
	containerName := c.Query("container")
	if containerName != "" {
		return containerName, nil
	}
	pod, err := client.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	if len(pod.Spec.Containers) == 0 {
		return "", errors.New("pod 没有可进入的容器")
	}
	return pod.Spec.Containers[0].Name, nil
}

func streamPodShell(c *gin.Context, client *kubernetes.Clientset, restConfig *rest.Config, terminal *websocketTerminal, namespace, podName, containerName string) error {
	shells := requestedShells(c.Query("shell"))
	var lastErr error
	for _, shell := range shells {
		err := execPodCommand(client, restConfig, terminal, namespace, podName, containerName, []string{shell})
		if err == nil {
			return nil
		}
		lastErr = err
	}
	return lastErr
}

func requestedShells(shell string) []string {
	switch shell {
	case "bash", "sh", "/bin/bash", "/bin/sh":
		return []string{shell}
	default:
		return []string{"bash", "sh", "/bin/bash", "/bin/sh"}
	}
}

func execPodCommand(client *kubernetes.Clientset, restConfig *rest.Config, terminal *websocketTerminal, namespace, podName, containerName string, command []string) error {
	req := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")

	req.VersionedParams(&v1.PodExecOptions{
		Container: containerName,
		Command:   command,
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
		TTY:       true,
	}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(restConfig, http.MethodPost, req.URL())
	if err != nil {
		return err
	}
	return executor.Stream(remotecommand.StreamOptions{
		Stdin:             terminal,
		Stdout:            terminal,
		Stderr:            terminal,
		TerminalSizeQueue: terminal,
		Tty:               true,
	})
}
