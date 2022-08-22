// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import "path"

const (
	perBuildWorkspace   = `$Env:WORKSPACE_DIR/$Env:DRONE_BUILD_NUMBER`
	windowsToolchainDir = perBuildWorkspace + `/toolchains`
	perBuildTeleportSrc = perBuildWorkspace + "/go/src/github.com/gravitational/teleport"
	perBuildWebappsSrc  = perBuildWorkspace + "/go/src/github.com/gravitational/webapps"

	// Hardcoded tool versions that we would normally pull from the makefile,
	// but unfortunately our Makefiles use too many POSIX-isms for us to use on
	// Windows, so for now we will just say the versions we want here.
	windowsGoVersion   = "1.18.3"
	windowsNodeVersion = "16.13.2"
)

func newWindowsPipeline(name string) pipeline {
	p := newExecPipeline(name)
	p.Workspace.Path = path.Join("C:/Drone/Workspace", name)
	p.Concurrency.Limit = 1
	p.Platform = platform{OS: "windows", Arch: "amd64"}
	return p
}

func windowsTagPipeline() pipeline {
	p := newWindowsPipeline("build-native-windows-amd64")

	//p.Trigger = triggerTag
	p.Trigger = trigger{
		Event:  triggerRef{Include: []string{"push"}, Exclude: []string{"pull_request"}},
		Branch: triggerRef{Include: []string{"master", "branch/*", "tcsc/build-windows*"}},
		Repo:   triggerRef{Include: []string{"gravitational/*"}},
	}

	// TODO(tcsc): restore before merge
	//p.DependsOn = []string{ "build-windows-amd64" }

	p.Steps = []step{
		cloneWindowsRepositoriesStep(p.Workspace.Path),
		updateWindowsSubreposStep(p.Workspace.Path),
		installWindowsNodeToolchainStep(p.Workspace.Path),
		installWindowsGoToolchainStep(p.Workspace.Path),
		buildWindowsTshStep(p.Workspace.Path),
		buildWindowsTeleportConnectStep(p.Workspace.Path),
		{
			Name: "Upload Artifacts",
			Environment: map[string]value{
				"WORKSPACE_DIR": {raw: p.Workspace.Path},
				// TODO(tcsc): aid during dev, revert before merge
				"AWS_REGION":            {raw: "ap-southeast-2"},
				"AWS_S3_BUCKET":         {raw: "trents-mock-dronestorage"},
				"AWS_ACCESS_KEY_ID":     {fromSecret: "MOCK_AWS_ACCESS_KEY_ID"},
				"AWS_SECRET_ACCESS_KEY": {fromSecret: "MOCK_AWS_SECRET_ACCESS_KEY"},
				// "AWS_REGION":            {raw: "us-west-2"},
				// "AWS_S3_BUCKET":         {fromSecret: "AWS_S3_BUCKET"},
				// "AWS_ACCESS_KEY_ID":     {fromSecret: "AWS_ACCESS_KEY_ID"},
				// "AWS_SECRET_ACCESS_KEY": {fromSecret: "AWS_SECRET_ACCESS_KEY"},
			},
			Commands: []string{
				`$Workspace = "` + perBuildWorkspace + `"`,
				`$TeleportSrc = "` + perBuildTeleportSrc + `"`,
				`$WebappsSrc = "` + perBuildWebappsSrc + `"`,
				`$Env:DRONE_TAG="v10.1.2"`, // TODO(tcsc): aid during dev, remove before merge
				`$TeleportVersion=$Env:DRONE_TAG.TrimStart('v')`,
				`$OutputsDir="$Workspace/outputs"`,
				`New-Item -Path "$OutputsDir" -ItemType 'Directory' | Out-Null`,
				`Get-ChildItem "$WebappsSrc/packages/teleterm/build/release`,
				`Copy-Item -Path "$WebappsSrc/packages/teleterm/build/release/Teleport Connect Setup*.exe" -Destination $OutputsDir`,
				`. "$TeleportSrc/build.assets/windows/build.ps1"`,
				`Format-FileHashes -PathGlob "$OutputsDir/*.exe"`,
				`Copy-Artifacts -Path $OutputsDir -Bucket $Env:AWS_S3_BUCKET -DstRoot "/teleport/tag/$TeleportVersion"`,
			},
		},
		cleanUpWindowsWorkspaceStep(p.Workspace.Path),
	}
	return p
}

func windowsPushPipeline() pipeline {
	p := newWindowsPipeline("push-build-native-windows-amd64")
	p.Trigger = trigger{
		Event:  triggerRef{Include: []string{"push"}, Exclude: []string{"pull_request"}},
		Branch: triggerRef{Include: []string{"master", "branch/*", "tcsc/build-windows-tcon"}},
		Repo:   triggerRef{Include: []string{"gravitational/*"}},
	}

	p.Steps = []step{
		cloneWindowsRepositoriesStep(p.Workspace.Path),
		updateWindowsSubreposStep(p.Workspace.Path),
		installWindowsNodeToolchainStep(p.Workspace.Path),
		installWindowsGoToolchainStep(p.Workspace.Path),
		buildWindowsTshStep(p.Workspace.Path),
		buildWindowsTeleportConnectStep(p.Workspace.Path),
		cleanUpWindowsWorkspaceStep(p.Workspace.Path),
	}

	return p
}

func cloneWindowsRepositoriesStep(workspace string) step {
	return step{
		Name: "Check out Teleport",
		Environment: map[string]value{
			"WORKSPACE_DIR": {raw: workspace},
		},
		Commands: []string{
			`$ErrorActionPreference = 'Stop'`,
			`$Env:GOCACHE = "` + perBuildWorkspace + `/gocache"`,
			`$TeleportSrc = "` + perBuildTeleportSrc + `"`,
			`$TeleportRev = if ($Env:DRONE_TAG -ne $null) { $Env:DRONE_TAG } else { $Env:DRONE_COMMIT }`,
			`New-Item -Path $TeleportSrc -ItemType Directory | Out-Null`,
			`cd $TeleportSrc`,
			`git clone https://github.com/gravitational/${DRONE_REPO_NAME}.git .`,
			`git checkout $TeleportRev`,
			`$WebappsSrc = "` + perBuildWebappsSrc + `"`,
			`New-Item -Path $WebappsSrc -ItemType Directory | Out-Null`,
			`cd $WebappsSrc`,
			`git clone https://github.com/gravitational/webapps.git .`,
			`git checkout $(go run $TeleportSrc/build.assets/tooling/cmd/get-webapps-version/main.go)`,
		},
	}
}

func updateWindowsSubreposStep(workspace string) step {
	return step{
		Name: "Checkout Submodules",
		Environment: map[string]value{
			"WORKSPACE_DIR":      {raw: workspace},
			"GITHUB_PRIVATE_KEY": {fromSecret: "GITHUB_PRIVATE_KEY"},
		},
		Commands: []string{
			`$Workspace = "` + perBuildWorkspace + `"`,
			`$TeleportSrc = "` + perBuildTeleportSrc + `"`,
			`. "$TeleportSrc/build.assets/windows/build.ps1"`,
			`Enable-Git -Workspace $Workspace -PrivateKey $Env:GITHUB_PRIVATE_KEY`,
			`cd $TeleportSrc`,
			`git submodule update --init e`,
			`git submodule update --init --recursive webassets`,
			`Reset-Git -Workspace $Workspace`,
		},
	}
}

func installWindowsNodeToolchainStep(workspacePath string) step {
	return step{
		Name:        "Install Node Toolchain",
		Environment: map[string]value{"WORKSPACE_DIR": {raw: workspacePath}},
		Commands: []string{
			`$ProgressPreference = 'SilentlyContinue'`,
			`$ErrorActionPreference = 'Stop'`,
			`$TeleportSrc = "` + perBuildTeleportSrc + `"`,
			`. "$TeleportSrc/build.assets/windows/build.ps1"`,
			// We can't use make, as there are too many posix dependencies in our makefile
			// to abstract away right now, so instead of `$(make -C $TeleportSrc/build.assets print-node-version)`,
			// we will just hardcode it for now
			`$NodeVersion = "` + windowsNodeVersion + `"`,
			`Install-Node -NodeVersion $NodeVersion -ToolchainDir "` + windowsToolchainDir + `"`,
		},
	}
}

func installWindowsGoToolchainStep(workspacePath string) step {
	return step{
		Name:        "Install Go Toolchain",
		Environment: map[string]value{"WORKSPACE_DIR": {raw: workspacePath}},
		Commands: []string{
			`$ProgressPreference = 'SilentlyContinue'`,
			`$ErrorActionPreference = 'Stop'`,
			`$TeleportSrc = "` + perBuildTeleportSrc + `"`,
			`. "$TeleportSrc/build.assets/windows/build.ps1"`,
			// We can't use make, as there are too many posix dependencies to
			// abstract away right now, so instead of `$(make -C $TeleportSrc/build.assets print-go-version)`,
			// we will just hardcode it for now
			`$GoVersion = "` + windowsGoVersion + `"`,
			`Install-Go -GoVersion $GoVersion -ToolchainDir "` + windowsToolchainDir + `"`,
		},
	}
}

func buildWindowsTshStep(workspace string) step {
	return step{
		Name: "Build tsh",
		Environment: map[string]value{
			"WORKSPACE_DIR":        {raw: workspace},
			"WINDOWS_SIGNING_CERT": {fromSecret: "WINDOWS_SIGNING_CERT"},
		},
		Commands: []string{
			`$Workspace = "` + perBuildWorkspace + `"`,
			`$Env:GOCACHE = "$Workspace/gocache"`,
			`$TeleportSrc = "` + perBuildTeleportSrc + `"`,
			`. "$TeleportSrc/build.assets/windows/build.ps1"`,
			`Enable-Go -ToolchainDir "` + windowsToolchainDir + `"`,
			`cd $TeleportSrc`,
			`$Env:GCO_ENABLED=1`,
			`go build -o build/tsh.exe ./tool/tsh`,
			`$Env:WINDOWS_SIGNING_CERT | File-Out -Encoding ASCII -Path windows-signing-cert.pfx`,
			`osslsigncode -pkcs12 windows-signing-cert.pfx -n Teleport -i https://goteleport.com -t http://timestamp.digicert.com -h sha2 -in build/tsh.exe -out build/tsh.signed.exe`,
			`Rename-Item -Force -Path build/tsh.signed.exe -NewName build/tsh.exe`,
		},
	}
}

func buildWindowsTeleportConnectStep(workspace string) step {
	return step{
		Name: "Build Teleport Connect",
		Environment: map[string]value{
			"WORKSPACE_DIR": {raw: workspace},
			"CSC_LINK":      {fromSecret: "WINDOWS_SIGNING_CERT"},
		},
		Commands: []string{
			`$Workspace = "` + perBuildWorkspace + `"`,
			`$TeleportSrc = "` + perBuildTeleportSrc + `"`,
			`$WebappsSrc = "` + perBuildWebappsSrc + `"`,
			`$NodeVersion = "` + windowsNodeVersion + `"`,
			`$Env:DRONE_TAG="v10.1.2`,
			`$TeleportVersion=$Env:DRONE_TAG.TrimStart('v')`,
			`. "$TeleportSrc/build.assets/windows/build.ps1"`,
			`Enable-Node -NodeVersion $NodeVersion -ToolchainDir "` + windowsToolchainDir + `"`,
			`cd $WebappsSrc`,
			`yarn install --frozen-lockfile`,
			`yarn build-term`,
			`yarn package-term -c.extraMetadata.version=$TeleportVersion`,
		},
	}
}

func cleanUpWindowsWorkspaceStep(workspacePath string) step {
	return step{
		Name:        "Clean up workspace (post)",
		Environment: map[string]value{"WORKSPACE_DIR": {raw: workspacePath}},
		When: &condition{
			Status: []string{"success", "failure"},
		},
		Commands: []string{
			// We don't want to break the build based on just a failed cleanup,
			// so we just tell PowerShell to carry on as best it can in the
			// face of an error
			`$ErrorActionPreference = 'Continue'`,
			`Remove-Item -Recurse -Force -Path "$Env:WORKSPACE_DIR/$Env:DRONE_BUILD_NUMBER"`,
		},
	}
}
