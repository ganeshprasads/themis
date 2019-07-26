// Copyright 2019 Comcast Cable Communications Management, LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bootstrap"
	"fmt"
	"key"
	"os"
	"random"
	"strings"
	"token"
	"xhealth"
	"xhttp/xhttpserver"
	"xlog/xloghttp"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

const (
	applicationName    = "themis"
	applicationVersion = "0.0.0"
)

func initialize(name string, arguments []string, fs *pflag.FlagSet, v *viper.Viper) error {
	var (
		file = fs.StringP("file", "f", "", "the configuration file to use.  Overrides the search path.")
		dev  = fs.BoolP("dev", "", false, "development node")
		iss  = fs.StringP("iss", "", "", "the name of the issuer to put into claims.  Overrides configuration.")
	)

	err := fs.Parse(arguments)
	if err != nil {
		return err
	}

	switch {
	case *dev:
		v.SetConfigType("yaml")
		err = v.ReadConfig(strings.NewReader(devMode))

	case len(*file) > 0:
		v.SetConfigFile(*file)
		err = v.ReadInConfig()

	default:
		v.SetConfigName(name)
		v.AddConfigPath(".")
		v.AddConfigPath(fmt.Sprintf("$HOME/.%s", name))
		v.AddConfigPath(fmt.Sprintf("/etc/%s", name))
		err = v.ReadInConfig()
	}

	if err != nil {
		return err
	}

	if len(*iss) > 0 {
		v.Set("issuer.claims.iss", *iss)
	}

	return nil
}

func main() {
	var (
		e = bootstrap.Environment{
			Name:       applicationName,
			LogKey:     "log",
			Initialize: initialize,
		}

		app = fx.New(
			e.Bootstrap(),
			provideMetrics("prometheus"),
			fx.Provide(
				xhealth.Unmarshal("health"),
				random.Provide,
				key.Provide,
				token.Unmarshal("token"),
				func() []xloghttp.ParameterBuilder {
					return []xloghttp.ParameterBuilder{
						xloghttp.Method("requestMethod"),
						xloghttp.URI("requestURI"),
						xloghttp.RemoteAddress("remoteAddr"),
					}
				},
				xhttpserver.ProvideParseForm,
				xhttpserver.UnmarshalResponseHeaders("responseHeaders"),
				provideClient("client"),
			),
			fx.Invoke(
				RunKeyServer("servers.key"),
				RunIssuerServer("servers.issuer"),
				RunClaimsServer("servers.claims"),
				RunMetricsServer("servers.metrics"),
				RunHealthServer("servers.health"),
				xhttpserver.InvokeOptional("servers.pprof", xhttpserver.AddPprofRoutes),
			),
		)
	)

	if err := app.Err(); err != nil {
		if err == pflag.ErrHelp {
			return
		}

		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	app.Run()
}