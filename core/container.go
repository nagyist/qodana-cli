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

package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/JetBrains/qodana-cli/v2025/cloud"
	"github.com/docker/docker/api/types/network"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/JetBrains/qodana-cli/v2025/core/corescan"
	"github.com/JetBrains/qodana-cli/v2025/platform/msg"
	"github.com/JetBrains/qodana-cli/v2025/platform/product"
	"github.com/JetBrains/qodana-cli/v2025/platform/qdcontainer"
	"github.com/JetBrains/qodana-cli/v2025/platform/qdenv"
	"github.com/JetBrains/qodana-cli/v2025/platform/strutil"
	"github.com/JetBrains/qodana-cli/v2025/platform/version"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/go-connections/nat"
	"github.com/pterm/pterm"

	cliconfig "github.com/docker/cli/cli/config"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
)

const (
	// officialImagePrefix is the prefix of official Qodana images.
	officialImagePrefix      = "jetbrains/qodana"
	dockerSpecialCharsLength = 8
	containerJvmDebugPort    = "5005"
)

var (
	containerLogsOptions = container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: false,
	}
	containerName = "qodana-cli"
)

// runQodanaContainer runs the analysis in a Docker container from a Qodana image.
func runQodanaContainer(ctx context.Context, c corescan.Context) int {
	dockerAnalyzer, ok := c.Analyser().(*product.DockerAnalyzer)
	if !ok {
		log.Fatalf("Context is not a DockerAnalyzer")
	}
	docker := qdcontainer.GetContainerClient()
	info, err := docker.Info(ctx)
	if err != nil {
		log.Fatal("Couldn't retrieve Docker daemon information", err)
	}
	if info.OSType != "linux" {
		msg.ErrorMessage("Container engine is not running a Linux platform, other platforms are not supported by Qodana")
		return 1
	}
	fixDarwinCaches(c.CacheDir())

	scanStages := getScanStages()

	image := dockerAnalyzer.Image
	CheckImage(image)
	if !c.SkipPull() {
		PullImage(docker, image)
	}
	progress, _ := msg.StartQodanaSpinner(scanStages[0])

	dockerConfig := getDockerOptions(c, image)
	log.Debugf("docker command to run: %s", generateDebugDockerRunCommand(dockerConfig))

	msg.UpdateText(progress, scanStages[1])

	runContainer(ctx, docker, dockerConfig)
	go followLinter(docker, dockerConfig.Name, progress, scanStages)

	exitCode := getContainerExitCode(ctx, docker, dockerConfig.Name)

	fixDarwinCaches(c.CacheDir())

	if progress != nil {
		_ = progress.Stop()
	}
	return int(exitCode)
}

// isUnofficialLinter checks if the linter is unofficial.
func isUnofficialLinter(linter string) bool {
	return !strings.HasPrefix(linter, officialImagePrefix)
}

// hasExactVersionTag checks if the linter has an exact version tag.
func hasExactVersionTag(linter string) bool {
	return strings.Contains(linter, ":") && !strings.Contains(linter, ":latest")
}

// isCompatibleLinter checks if the linter is compatible with the current CLI.
func isCompatibleLinter(linter string) bool {
	return strings.Contains(linter, product.ReleaseVersion)
}

// CheckImage checks the linter image and prints warnings if necessary.
func CheckImage(linter string) {
	if strings.Contains(version.Version, "nightly") || strings.Contains(version.Version, "dev") {
		return
	}

	if isUnofficialLinter(linter) {
		msg.WarningMessageCI("You are using an unofficial Qodana linter: %s\n", linter)
	}

	if !hasExactVersionTag(linter) {
		msg.WarningMessageCI(
			"You are running a Qodana linter without an exact version tag: %s \n   Consider pinning the version in your configuration to ensure version compatibility: %s\n",
			linter,
			strings.Join([]string{strutil.SafeSplit(linter, ":", 0), product.ReleaseVersion}, ":"),
		)
	} else if !isCompatibleLinter(linter) {
		msg.WarningMessageCI(
			"You are using a non-compatible Qodana linter %s with the current CLI (%s) \n   Consider updating CLI or using a compatible linter %s \n",
			linter,
			version.Version,
			strings.Join([]string{strutil.SafeSplit(linter, ":", 0), product.ReleaseVersion}, ":"),
		)
	}
}

func fixDarwinCaches(cacheDir string) {
	if //goland:noinspection GoBoolExpressions
	runtime.GOOS == "darwin" {
		err := removePortSocket(cacheDir)
		if err != nil {
			log.Warnf("Could not remove .port from %s: %s", cacheDir, err)
		}
	}
}

// removePortSocket removes .port from the system dir to resolve QD-7383.
func removePortSocket(systemDir string) error {
	ideaDir := filepath.Join(systemDir, "idea")
	files, err := os.ReadDir(ideaDir)
	if err != nil {
		return nil
	}
	for _, file := range files {
		if file.IsDir() {
			dotPort := filepath.Join(ideaDir, file.Name(), ".port")
			if _, err = os.Stat(dotPort); err == nil {
				err = os.Remove(dotPort)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// encodeAuthToBase64 serializes the auth configuration as JSON base64 payload
func encodeAuthToBase64(authConfig registry.AuthConfig) (string, error) {
	buf, err := json.Marshal(authConfig)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}

// PullImage pulls docker image and prints the process.
func PullImage(client *client.Client, image string) {
	msg.PrintProcess(
		func(_ *pterm.SpinnerPrinter) {
			pullImage(context.Background(), client, image)
		},
		fmt.Sprintf("Pulling the image %s", msg.PrimaryBold(image)),
		"pulling the latest version of linter",
	)
}

func isDockerUnauthorizedError(errMsg string) bool {
	errMsg = strutil.Lower(errMsg)
	return strings.Contains(errMsg, "unauthorized") || strings.Contains(errMsg, "denied") || strings.Contains(
		errMsg,
		"forbidden",
	)
}

// pullImage pulls docker image.
func pullImage(ctx context.Context, client *client.Client, image string) {
	reader, err := client.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil && isDockerUnauthorizedError(err.Error()) {
		cfg, err := cliconfig.Load("")
		if err != nil {
			log.Fatal(err)
		}

		registryHostname := strutil.SafeSplit(image, "/", 0)

		a, err := cfg.GetAuthConfig(registryHostname)
		if err != nil {
			log.Fatal("can't load the auth config", err)
		}
		encodedAuth, err := encodeAuthToBase64(registry.AuthConfig(a))
		if err != nil {
			log.Fatal("can't encode auth to base64", err)
		}
		reader, err = client.ImagePull(ctx, image, types.ImagePullOptions{RegistryAuth: encodedAuth})
		if err != nil {
			log.Fatal("can't pull image from the private registry", err)
		}
	} else if err != nil {
		log.Fatal("can't pull image ", err)
	}
	defer func(pull io.ReadCloser) {
		err := pull.Close()
		if err != nil {
			log.Fatal("can't pull image ", err)
		}
	}(reader)
	if _, err = io.Copy(io.Discard, reader); err != nil {
		log.Fatal("couldn't read the image pull logs ", err)
	}
}

// ContainerCleanup cleans up Qodana containers.
func ContainerCleanup() {
	if containerName != "qodana-cli" { // if containerName is not set, it means that the container was not created!
		docker := qdcontainer.GetContainerClient()
		ctx := context.Background()
		containers, err := docker.ContainerList(ctx, container.ListOptions{})
		if err != nil {
			log.Fatal("couldn't get the running containers ", err)
		}
		for _, c := range containers {
			if c.Names[0] == fmt.Sprintf("/%s", containerName) {
				err = docker.ContainerStop(context.Background(), c.Names[0], container.StopOptions{})
				if err != nil {
					log.Fatal("couldn't stop the container ", err)
				}
			}
		}
	}
}

// CheckContainerEngineMemory applicable only for Docker Desktop,
// (has the default limit of 2GB which can be not enough when Gradle runs inside a container).
func CheckContainerEngineMemory() {
	qdcontainer.CheckContainerEngineMemory()
}

// getDockerOptions returns qodana docker container options.
func getDockerOptions(c corescan.Context, image string) *backend.ContainerCreateConfig {
	cmdOpts := GetIdeArgs(c)

	updateScanContextEnv := func(key string, value string) { c = c.WithEnvExtractedFromOsEnv(key, value) }
	qdenv.ExtractQodanaEnvironment(updateScanContextEnv)

	dockerEnv := c.Env()
	qodanaCloudUploadToken := c.QodanaUploadToken()
	if qodanaCloudUploadToken != "" {
		dockerEnv = append(dockerEnv, fmt.Sprintf("%s=%s", qdenv.QodanaToken, qodanaCloudUploadToken))
	}
	qodanaLicenseOnlyToken := os.Getenv(qdenv.QodanaLicenseOnlyToken)
	if qodanaLicenseOnlyToken != "" && qodanaCloudUploadToken == "" {
		dockerEnv = append(dockerEnv, fmt.Sprintf("%s=%s", qdenv.QodanaLicenseOnlyToken, qodanaLicenseOnlyToken))
	}

	cachePath, err := filepath.Abs(c.CacheDir())
	if err != nil {
		log.Fatal("couldn't get abs path for cache", err)
	}
	projectPath, err := filepath.Abs(c.ProjectDir())
	if err != nil {
		log.Fatal("couldn't get abs path for project", err)
	}
	resultsPath, err := filepath.Abs(c.ResultsDir())
	if err != nil {
		log.Fatal("couldn't get abs path for results", err)
	}
	reportPath, err := filepath.Abs(c.ReportDir())
	if err != nil {
		log.Fatal("couldn't get abs path for report", err)
	}
	containerName = os.Getenv(qdenv.QodanaCliContainerName)
	if containerName == "" {
		containerName = fmt.Sprintf("qodana-cli-%s", c.Id())
	}
	volumes := []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: cachePath,
			Target: qdcontainer.DataCacheDir,
		},
		{
			Type:   mount.TypeBind,
			Source: projectPath,
			Target: qdcontainer.DataProjectDir,
		},
		{
			Type:   mount.TypeBind,
			Source: resultsPath,
			Target: qdcontainer.DataResultsDir,
		},
		{
			Type:   mount.TypeBind,
			Source: reportPath,
			Target: qdcontainer.DataResultsReportDir,
		},
	}
	if c.GlobalConfigurationsDir() != "" {
		globalConfigDirAbsPath, err := filepath.Abs(c.GlobalConfigurationsDir())
		if err != nil {
			log.Fatalf(
				"Failed to get absolute path for global configurations file %s: %s",
				c.GlobalConfigurationsDir(),
				err,
			)
		}
		volumes = append(
			volumes, mount.Mount{
				Type:   mount.TypeBind,
				Source: globalConfigDirAbsPath,
				Target: qdcontainer.DataGlobalConfigDir,
			},
		)
	}
	for _, volume := range c.Volumes() {
		source, target := extractDockerVolumes(volume)
		if source != "" && target != "" {
			volumes = append(
				volumes, mount.Mount{
					Type:   mount.TypeBind,
					Source: source,
					Target: target,
				},
			)
		} else {
			log.Fatal("couldn't parse volume ", volume)
		}
	}
	log.Debugf("image: %s", image)
	log.Debugf("container name: %s", containerName)
	log.Debugf("user: %s", c.User())
	log.Debugf("volumes: %v", volumes)
	log.Debugf("cmd: %v", cmdOpts)

	portBindings := make(nat.PortMap)
	exposedPorts := make(nat.PortSet)

	if c.JvmDebugPort() > 0 {
		log.Infof("Enabling JVM debug on port %d", c.JvmDebugPort())
		portBindings = nat.PortMap{
			containerJvmDebugPort: []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: strconv.Itoa(c.JvmDebugPort()),
				},
			},
		}
		exposedPorts = nat.PortSet{
			containerJvmDebugPort: struct{}{},
		}
	}

	var capAdd []string
	var securityOpt []string
	var networkMode container.NetworkMode

	if strings.Contains(image, "dotnet") {
		capAdd = []string{"SYS_PTRACE"}
		securityOpt = []string{"seccomp=unconfined"}
	}

	// See QD-11584 for reasoning
	//goland:noinspection HttpUrlsUsage
	isLocalHttpCloud := strings.HasPrefix(cloud.GetCloudRootEndpoint().Url, "http://")
	if isLocalHttpCloud {
		networkMode = network.NetworkHost
	}

	var hostConfig = &container.HostConfig{
		AutoRemove:   os.Getenv(qdenv.QodanaCliContainerKeep) == "",
		Mounts:       volumes,
		CapAdd:       capAdd,
		SecurityOpt:  securityOpt,
		PortBindings: portBindings,
		NetworkMode:  networkMode,
	}

	return &backend.ContainerCreateConfig{
		Name: containerName,
		Config: &container.Config{
			Image:        image,
			Cmd:          cmdOpts,
			Tty:          msg.IsInteractive(),
			AttachStdout: true,
			AttachStderr: true,
			Env:          dockerEnv,
			User:         c.User(),
			ExposedPorts: exposedPorts,
		},
		HostConfig: hostConfig,
	}
}

func generateDebugDockerRunCommand(cfg *backend.ContainerCreateConfig) string {
	var cmdBuilder strings.Builder
	cmdBuilder.WriteString("docker run ")
	if cfg.HostConfig != nil && cfg.HostConfig.AutoRemove {
		cmdBuilder.WriteString("--rm ")
	}
	if cfg.Config.AttachStdout {
		cmdBuilder.WriteString("-a stdout ")
	}
	if cfg.Config.AttachStderr {
		cmdBuilder.WriteString("-a stderr ")
	}
	if cfg.Config.Tty {
		cmdBuilder.WriteString("-it ")
	}
	if cfg.Config.User != "" {
		cmdBuilder.WriteString(fmt.Sprintf("-u %s ", cfg.Config.User))
	}
	for _, env := range cfg.Config.Env {
		if !strings.Contains(env, qdenv.QodanaToken) || strings.Contains(
			env,
			qdenv.QodanaLicense,
		) || strings.Contains(env, qdenv.QodanaLicenseOnlyToken) {
			cmdBuilder.WriteString(fmt.Sprintf("-e %s ", env))
		}
	}
	if cfg.HostConfig != nil {
		for _, m := range cfg.HostConfig.Mounts {
			cmdBuilder.WriteString(fmt.Sprintf("-v %s:%s ", m.Source, m.Target))
		}
		for _, capAdd := range cfg.HostConfig.CapAdd {
			cmdBuilder.WriteString(fmt.Sprintf("--cap-add %s ", capAdd))
		}
		for _, secOpt := range cfg.HostConfig.SecurityOpt {
			cmdBuilder.WriteString(fmt.Sprintf("--security-opt %s ", secOpt))
		}
	}
	cmdBuilder.WriteString(cfg.Config.Image + " ")
	for _, arg := range cfg.Config.Cmd {
		cmdBuilder.WriteString(fmt.Sprintf("%s ", arg))
	}

	return cmdBuilder.String()
}

// getContainerExitCode returns the exit code of the docker container.
func getContainerExitCode(ctx context.Context, client *client.Client, id string) int64 {
	statusCh, errCh := client.ContainerWait(ctx, id, container.WaitConditionNextExit)
	select {
	case err := <-errCh:
		if err != nil {
			log.Fatal("container hasn't finished ", err)
		}
	case status := <-statusCh:
		return status.StatusCode
	}
	return 0
}

// runContainer runs the container.
func runContainer(ctx context.Context, client *client.Client, opts *backend.ContainerCreateConfig) {
	createResp, err := client.ContainerCreate(
		ctx,
		opts.Config,
		opts.HostConfig,
		nil,
		nil,
		opts.Name,
	)
	if err != nil {
		log.Fatal("couldn't create the container ", err)
	}
	if err = client.ContainerStart(ctx, createResp.ID, container.StartOptions{}); err != nil {
		log.Fatal("couldn't bootstrap the container ", err)
	}
}

// extractDockerVolumes extracts the source and target of the volume to mount.
func extractDockerVolumes(volume string) (string, string) {
	if //goland:noinspection GoBoolExpressions
	runtime.GOOS == "windows" {
		parts := strings.Split(volume, ":")
		if len(parts) >= 3 {
			return fmt.Sprintf("%s:%s", parts[0], parts[1]), parts[2]
		}
	} else {
		source := strutil.SafeSplit(volume, ":", 0)
		target := strutil.SafeSplit(volume, ":", 1)
		if source != "" && target != "" {
			return source, target
		}
	}
	return "", ""
}
