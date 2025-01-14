/*
Copyright 2021 The Topolvm-Operator Authors. All rights reserved.

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

package cluster

import "github.com/coreos/pkg/capnslog"

var (
	LogLevelRaw string
	Cfg         = &Config{}
)

type Config struct {
	LogLevel capnslog.LogLevel
}

// SetLogLevel set log level based on provided log option.
func SetLogLevel() {
	// parse given log level string then set up corresponding global logging level
	ll, err := capnslog.ParseLevel(LogLevelRaw)
	if err != nil {
		logger.Warningf("failed to set log level %s. %+v", LogLevelRaw, err)
	}
	Cfg.LogLevel = ll
	capnslog.SetGlobalLogLevel(Cfg.LogLevel)
}
