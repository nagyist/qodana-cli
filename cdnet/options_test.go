/*
 * Copyright 2021-2024 JetBrains s.r.o.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"fmt"
	"github.com/JetBrains/qodana-cli/v2025/core"
	"github.com/JetBrains/qodana-cli/v2025/core/corescan"
	"github.com/JetBrains/qodana-cli/v2025/platform/product"
	"github.com/JetBrains/qodana-cli/v2025/platform/qdcontainer"
	"github.com/JetBrains/qodana-cli/v2025/platform/qdyaml"
	"github.com/JetBrains/qodana-cli/v2025/platform/strutil"
	"github.com/JetBrains/qodana-cli/v2025/platform/thirdpartyscan"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func createDefaultYaml(sln string, prj string, cfg string, plt string) thirdpartyscan.QodanaYamlConfig {
	return thirdpartyscan.QodanaYamlConfig{
		DotNet: qdyaml.DotNet{
			Solution:      sln,
			Project:       prj,
			Configuration: cfg,
			Platform:      plt,
		},
	}
}

func TestComputeCdnetArgs(t *testing.T) {
	tests := []struct {
		name         string
		cb           thirdpartyscan.ContextBuilder
		expectedArgs []string
		expectedErr  string
	}{
		{
			name: "No solution/project specified",
			cb: thirdpartyscan.ContextBuilder{
				Property:         []string{},
				ResultsDir:       "",
				QodanaYamlConfig: createDefaultYaml("", "", "", ""),
			},
			expectedArgs: nil,
			expectedErr:  "solution/project relative file path is not specified. Use --solution or --project flags or create qodana.yaml file with respective fields",
		},
		{
			name: "project specified",
			cb: thirdpartyscan.ContextBuilder{
				Property:         []string{},
				ResultsDir:       "",
				CdnetProject:     "project",
				QodanaYamlConfig: createDefaultYaml("", "", "", ""),
			},
			expectedArgs: []string{
				"dotnet",
				"clt",
				"inspectcode",
				"project",
				"-o=\"qodana.sarif.json\"",
				"-f=\"Qodana\"",
				"--LogFolder=\"log\"",
			},
			expectedErr: "",
		},
		{
			name: "project specified in yaml",
			cb: thirdpartyscan.ContextBuilder{
				Property:         []string{},
				ResultsDir:       "",
				QodanaYamlConfig: createDefaultYaml("", "project", "", ""),
			},
			expectedArgs: []string{
				"dotnet",
				"clt",
				"inspectcode",
				"project",
				"-o=\"qodana.sarif.json\"",
				"-f=\"Qodana\"",
				"--LogFolder=\"log\"",
			},
			expectedErr: "",
		},
		{
			name: "solution specified",
			cb: thirdpartyscan.ContextBuilder{
				Property:         []string{},
				ResultsDir:       "",
				CdnetSolution:    "solution",
				QodanaYamlConfig: createDefaultYaml("", "", "", ""),
			},
			expectedArgs: []string{
				"dotnet",
				"clt",
				"inspectcode",
				"solution",
				"-o=\"qodana.sarif.json\"",
				"-f=\"Qodana\"",
				"--LogFolder=\"log\"",
			},
			expectedErr: "",
		},
		{
			name: "solution specified",
			cb: thirdpartyscan.ContextBuilder{
				Property:         []string{},
				ResultsDir:       "",
				QodanaYamlConfig: createDefaultYaml("solution", "", "", ""),
			},
			expectedArgs: []string{
				"dotnet",
				"clt",
				"inspectcode",
				"solution",
				"-o=\"qodana.sarif.json\"",
				"-f=\"Qodana\"",
				"--LogFolder=\"log\"",
			},
			expectedErr: "",
		},
		{
			name: "configuration specified in yaml",
			cb: thirdpartyscan.ContextBuilder{
				Property:         []string{},
				ResultsDir:       "",
				QodanaYamlConfig: createDefaultYaml("solution", "", "cfg", ""),
			},
			expectedArgs: []string{
				"dotnet",
				"clt",
				"inspectcode",
				"solution",
				"-o=\"qodana.sarif.json\"",
				"-f=\"Qodana\"",
				"--LogFolder=\"log\"",
				"--properties:Configuration=cfg",
			},
			expectedErr: "",
		},
		{
			name: "configuration specified",
			cb: thirdpartyscan.ContextBuilder{
				Property:           []string{},
				ResultsDir:         "",
				CdnetConfiguration: "cfg",
				QodanaYamlConfig:   createDefaultYaml("solution", "", "", ""),
			},
			expectedArgs: []string{
				"dotnet",
				"clt",
				"inspectcode",
				"solution",
				"-o=\"qodana.sarif.json\"",
				"-f=\"Qodana\"",
				"--LogFolder=\"log\"",
				"--properties:Configuration=cfg",
			},
			expectedErr: "",
		},
		{
			name: "platform specified in cfg",
			cb: thirdpartyscan.ContextBuilder{
				Property:         []string{},
				ResultsDir:       "",
				QodanaYamlConfig: createDefaultYaml("solution", "", "", "x64"),
			},
			expectedArgs: []string{
				"dotnet",
				"clt",
				"inspectcode",
				"solution",
				"-o=\"qodana.sarif.json\"",
				"-f=\"Qodana\"",
				"--LogFolder=\"log\"",
				"--properties:Platform=x64",
			},
			expectedErr: "",
		},
		{
			name: "platform specified",
			cb: thirdpartyscan.ContextBuilder{
				Property:         []string{},
				ResultsDir:       "",
				CdnetPlatform:    "x64",
				QodanaYamlConfig: createDefaultYaml("solution", "", "", ""),
			},
			expectedArgs: []string{
				"dotnet",
				"clt",
				"inspectcode",
				"solution",
				"-o=\"qodana.sarif.json\"",
				"-f=\"Qodana\"",
				"--LogFolder=\"log\"",
				"--properties:Platform=x64",
			},
			expectedErr: "",
		},
		{
			name: "many options",
			cb: thirdpartyscan.ContextBuilder{
				Property:           []string{"prop1=val1", "prop2=val2"},
				ResultsDir:         "",
				CdnetPlatform:      "x64",
				CdnetConfiguration: "Debug",
				QodanaYamlConfig:   createDefaultYaml("solution", "", "", ""),
			},
			expectedArgs: []string{
				"dotnet",
				"clt",
				"inspectcode",
				"solution",
				"-o=\"qodana.sarif.json\"",
				"-f=\"Qodana\"",
				"--LogFolder=\"log\"",
				"--properties:prop1=val1;prop2=val2;Configuration=Debug;Platform=x64",
			},
			expectedErr: "",
		},
		{
			name: "no-build",
			cb: thirdpartyscan.ContextBuilder{
				Property:         []string{},
				ResultsDir:       "",
				CdnetNoBuild:     true,
				QodanaYamlConfig: createDefaultYaml("solution", "", "", ""),
			},
			expectedArgs: []string{
				"dotnet",
				"clt",
				"inspectcode",
				"solution",
				"-o=\"qodana.sarif.json\"",
				"-f=\"Qodana\"",
				"--LogFolder=\"log\"",
				"--no-build",
			},
			expectedErr: "",
		},
		{
			name: "TeamCity args ignored",
			cb: thirdpartyscan.ContextBuilder{
				Property: []string{
					"log.project.structure.changes=true",
					"idea.log.config.file=warn.xml",
					"qodana.default.file.suspend.threshold=100000",
					"qodana.default.module.suspend.threshold=100000",
					"qodana.default.project.suspend.threshold=100000",
					fmt.Sprintf(
						"idea.diagnostic.opentelemetry.file=%s/log/open-telemetry.json",
						qdcontainer.DataResultsDir,
					),
					"jetbrains.security.package-checker.synchronizationTimeout=1000",
				},
				ResultsDir:       "",
				QodanaYamlConfig: createDefaultYaml("solution", "", "", ""),
			},
			expectedArgs: []string{
				"dotnet",
				"clt",
				"inspectcode",
				"solution",
				"-o=\"qodana.sarif.json\"",
				"-f=\"Qodana\"",
				"--LogFolder=\"log\"",
			},
			expectedErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				logDir := "logDir"

				tt.cb.LogDir = logDir
				tt.cb.MountInfo = getTooling()
				context := tt.cb.Build()
				args, err := CdnetLinter{}.computeCdnetArgs(context)

				if strutil.Contains(tt.expectedArgs, "--LogFolder=\"log\"") {
					for i, arg := range tt.expectedArgs {
						if arg == "--LogFolder=\"log\"" {
							tt.expectedArgs[i] = "--LogFolder=\"" + logDir + "\""
						}
					}

				}

				if tt.expectedErr != "" {
					assert.NotNil(t, err)
					assert.Equal(t, tt.expectedErr, err.Error())
				} else {
					assert.Nil(t, err)
					assert.Equal(t, tt.expectedArgs, args)
				}
			},
		)
	}
}

func getTooling() thirdpartyscan.MountInfo {
	return thirdpartyscan.MountInfo{
		CustomTools: map[string]string{"clt": "clt"},
	}
}

func TestGetArgsThirdPartyLinters(t *testing.T) {
	cases := []struct {
		name     string
		cb       corescan.ContextBuilder
		expected []string
	}{
		{
			name: "not sending statistics",
			cb: corescan.ContextBuilder{
				NoStatistics: true,
				Analyser:     product.DotNetCommunityLinter.DockerAnalyzer(),
			},
			expected: []string{
				"--no-statistics",
			},
		},
		{
			name: "(cdnet) solution",
			cb: corescan.ContextBuilder{
				CdnetSolution: "solution.sln",
				Analyser:      product.DotNetCommunityLinter.DockerAnalyzer(),
			},
			expected: []string{
				"--solution", "solution.sln",
			},
		},
		{
			name: "(cdnet) project",
			cb: corescan.ContextBuilder{
				CdnetProject: "project.csproj",
				Analyser:     product.DotNetCommunityLinter.DockerAnalyzer(),
			},
			expected: []string{
				"--project", "project.csproj",
			},
		},
		{
			name: "(cdnet) configuration",
			cb: corescan.ContextBuilder{
				CdnetConfiguration: "Debug",
				Analyser:           product.DotNetCommunityLinter.DockerAnalyzer(),
			},
			expected: []string{
				"--configuration", "Debug",
			},
		},
		{
			name: "(cdnet) platform",
			cb: corescan.ContextBuilder{
				CdnetPlatform: "x64",
				Analyser:      product.DotNetCommunityLinter.DockerAnalyzer(),
			},
			expected: []string{
				"--platform", "x64",
			},
		},
		{
			name: "(cdnet) no build",
			cb: corescan.ContextBuilder{
				CdnetNoBuild: true,
				Analyser:     product.DotNetCommunityLinter.DockerAnalyzer(),
			},
			expected: []string{
				"--no-build",
			},
		},
		{
			name: "(clang) compile commands",
			cb: corescan.ContextBuilder{
				ClangCompileCommands: "compile_commands.json",
				Analyser:             product.ClangLinter.DockerAnalyzer(),
			},
			expected: []string{
				"--compile-commands", "compile_commands.json",
			},
		},
		{
			name: "(clang) clang args",
			cb: corescan.ContextBuilder{
				ClangArgs: "-I/usr/include",
				Analyser:  product.ClangLinter.DockerAnalyzer(),
			},
			expected: []string{
				"--clang-args", "-I/usr/include",
			},
		},
		{
			name: "using flag in non 3rd party linter",
			cb: corescan.ContextBuilder{
				NoStatistics: true,
				Analyser:     product.DotNetLinter.NativeAnalyzer(),
			},
			expected: []string{},
		},
	}

	for _, tt := range cases {
		t.Run(
			tt.name, func(t *testing.T) {
				contextBuilder := tt.cb

				context := contextBuilder.Build()
				actual := core.GetIdeArgs(context)
				if !reflect.DeepEqual(tt.expected, actual) {
					t.Fatalf("expected \"%s\" got \"%s\"", tt.expected, actual)
				}
			},
		)
	}
}
