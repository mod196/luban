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

package services

import (
	"github.com/casbin/casbin/util"
	"github.com/casbin/casbin/v2"
	gormAdapter "github.com/casbin/gorm-adapter/v3"
	"github.com/dnsjia/luban/common"
	_ "github.com/go-sql-driver/mysql"
	"strings"
	"sync"
)

var (
	casbinMu       sync.Mutex
	casbinEnforcer *casbin.SyncedEnforcer
)

func Casbin() (*casbin.SyncedEnforcer, error) {
	casbinMu.Lock()
	defer casbinMu.Unlock()

	if casbinEnforcer != nil {
		return casbinEnforcer, nil
	}

	admin := common.CONFIG.Mysql
	a, err := gormAdapter.NewAdapter(common.CONFIG.System.DbType, admin.Username+":"+admin.Password+"@("+admin.Path+")/"+admin.Dbname, true)
	if err != nil {
		return nil, err
	}
	e, err := casbin.NewSyncedEnforcer(common.CONFIG.Casbin.ModelPath, a)
	if err != nil {
		return nil, err
	}
	e.AddFunction("ParamsMatch", ParamsMatchFunc)
	if err := e.LoadPolicy(); err != nil {
		return nil, err
	}
	casbinEnforcer = e
	return casbinEnforcer, nil
}

func ReloadCasbinPolicy() error {
	e, err := Casbin()
	if err != nil {
		return err
	}
	return e.LoadPolicy()
}

func ParamsMatch(fullNameKey1 string, key2 string) bool {
	/*
		自定义规则函数
		fullNameKey1 string, key2 string
	*/
	key1 := strings.Split(fullNameKey1, "?")[0]
	// 剥离路径后再使用casbin的keyMatch2
	return util.KeyMatch2(key1, key2)
}

func ParamsMatchFunc(args ...interface{}) (interface{}, error) {
	/*
		自定义规则函数
	*/
	name1 := args[0].(string)
	name2 := args[1].(string)

	return ParamsMatch(name1, name2), nil
}
