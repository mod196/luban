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

package common

import (
	"bytes"
	"fmt"
	"github.com/dnsjia/luban/models"
	"github.com/dnsjia/luban/pkg/iconf"
	"github.com/spf13/viper"
	"github.com/toolkits/pkg/file"
)

type Server struct {
	Zap     Zap           `mapstructure:"zap"    json:"zap" yaml:"zap"`
	Mysql   Mysql         `mapstructure:"mysql"  json:"mysql" yaml:"mysql"`
	Casbin  models.Casbin `mapstructure:"casbin" json:"casbin" yaml:"casbin"`
	System  System        `mapstructure:"system" json:"system" yaml:"system"`
	HTTP    HTTP          `mapstructure:"http"   json:"http" yaml:"http"`
	Redis   Redis         `mapstructure:"redis"  json:"redis" yaml:"redis"`
	Crontab Crontab       `mapstructure:"crontab" json:"crontab" yaml:"crontab"`
}

type HTTP struct {
	Mode   string `mapstructure:"mode" json:"mode" yaml:"mode"`
	Listen int    `mapstructure:"listen" json:"listen" yaml:"listen"`
}

type contactKey struct {
	Label string `yaml:"label" json:"label"`
	Key   string `yaml:"key" json:"key"`
}

type ConfigStruct struct {
	LDAP models.LdapSection `yaml:"ldap"`
}

var Config *ConfigStruct

func NormalizeRuntimeConfig() {
	if CONFIG.System.PlatformTitle == "" {
		CONFIG.System.PlatformTitle = "运维平台"
	}
	if CONFIG.System.Addr == 0 && CONFIG.HTTP.Listen > 0 {
		CONFIG.System.Addr = CONFIG.HTTP.Listen
	}
	if CONFIG.System.Addr == 0 {
		CONFIG.System.Addr = 8999
	}
	if CONFIG.System.Env == "" && CONFIG.HTTP.Mode != "" {
		CONFIG.System.Env = CONFIG.HTTP.Mode
	}
	if CONFIG.System.DbType == "" {
		CONFIG.System.DbType = "mysql"
	}
	if CONFIG.Mysql.Config == "" {
		CONFIG.Mysql.Config = "charset=utf8mb4&parseTime=True&loc=Local"
	}
	if CONFIG.Casbin.ModelPath == "" {
		CONFIG.Casbin.ModelPath = "./etc/rbac_model.conf"
	}
}

func Parse() error {
	ymlFile := iconf.GetYmlFile("server")
	if ymlFile == "" {
		return fmt.Errorf("configuration file of server not found")
	}

	bs, err := file.ReadBytes(ymlFile)
	if err != nil {
		return fmt.Errorf("cannot read yml[%s]: %v", ymlFile, err)
	}

	viper.SetConfigType("yaml")
	err = viper.ReadConfig(bytes.NewBuffer(bs))
	if err != nil {
		return fmt.Errorf("cannot read yml[%s]: %v", ymlFile, err)
	}
	err = viper.Unmarshal(&Config)
	if err != nil {
		return fmt.Errorf("cannot read yml[%s]: %v", ymlFile, err)
	}

	fmt.Println("config.file:", ymlFile)

	return nil
}
