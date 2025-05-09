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
	"github.com/JetBrains/qodana-cli/v2025/cmd"
	"github.com/JetBrains/qodana-cli/v2025/core"
	"github.com/JetBrains/qodana-cli/v2025/platform/commoncontext"
	"github.com/JetBrains/qodana-cli/v2025/platform/msg"
	"github.com/JetBrains/qodana-cli/v2025/platform/version"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	commoncontext.InterruptChannel = make(chan os.Signal, 1)
	signal.Notify(commoncontext.InterruptChannel, os.Interrupt)
	signal.Notify(commoncontext.InterruptChannel, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-commoncontext.InterruptChannel
		msg.WarningMessage("Interrupting Qodana CLI...")
		log.SetOutput(io.Discard)
		core.CheckForUpdates(version.Version)
		core.ContainerCleanup()
		_ = msg.QodanaSpinner.Stop()
		os.Exit(0)
	}()
	cmd.InitCli()
	cmd.Execute()
}
