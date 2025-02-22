package docker

import (
	"bufio"
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"

	//"github.com/docker/go-connections/nat"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/flyteorg/flytectl/pkg/docker/mocks"
	"github.com/stretchr/testify/mock"

	"github.com/docker/docker/api/types"
	cmdCore "github.com/flyteorg/flytectl/cmd/core"
	u "github.com/flyteorg/flytectl/cmd/testutils"

	f "github.com/flyteorg/flytectl/pkg/filesystemutils"

	"github.com/stretchr/testify/assert"
)

var (
	cmdCtx     cmdCore.CommandContext
	containers []types.Container
)

func setupSandbox() {
	mockAdminClient := u.MockClient
	cmdCtx = cmdCore.NewCommandContext(mockAdminClient, u.MockOutStream)
	_ = SetupFlyteDir()
	container1 := types.Container{
		ID: "FlyteSandboxClusterName",
		Names: []string{
			FlyteSandboxClusterName,
		},
	}
	containers = append(containers, container1)
}

func TestConfigCleanup(t *testing.T) {
	_, err := os.Stat(f.FilePathJoin(f.UserHomeDir(), ".flyte"))
	if os.IsNotExist(err) {
		_ = os.MkdirAll(f.FilePathJoin(f.UserHomeDir(), ".flyte"), 0755)
	}
	_ = ioutil.WriteFile(FlytectlConfig, []byte("string"), 0600)
	_ = ioutil.WriteFile(Kubeconfig, []byte("string"), 0600)

	err = ConfigCleanup()
	assert.Nil(t, err)

	_, err = os.Stat(FlytectlConfig)
	check := os.IsNotExist(err)
	assert.Equal(t, check, true)

	_, err = os.Stat(Kubeconfig)
	check = os.IsNotExist(err)
	assert.Equal(t, check, true)
	_ = ConfigCleanup()
}

func TestSetupFlytectlConfig(t *testing.T) {
	_, err := os.Stat(f.FilePathJoin(f.UserHomeDir(), ".flyte"))
	if os.IsNotExist(err) {
		_ = os.MkdirAll(f.FilePathJoin(f.UserHomeDir(), ".flyte"), 0755)
	}
	err = SetupFlyteDir()
	assert.Nil(t, err)
	err = GetFlyteSandboxConfig()
	assert.Nil(t, err)
	_, err = os.Stat(FlytectlConfig)
	assert.Nil(t, err)
	check := os.IsNotExist(err)
	assert.Equal(t, check, false)
	_ = ConfigCleanup()
}

func TestGetSandbox(t *testing.T) {
	setupSandbox()
	t.Run("Successfully get sandbox container", func(t *testing.T) {
		mockDocker := &mocks.Docker{}
		context := context.Background()

		mockDocker.OnContainerList(context, types.ContainerListOptions{All: true}).Return(containers, nil)
		c := GetSandbox(context, mockDocker)
		assert.Equal(t, c.Names[0], FlyteSandboxClusterName)
	})

	t.Run("Successfully get sandbox container with zero result", func(t *testing.T) {
		mockDocker := &mocks.Docker{}
		context := context.Background()

		mockDocker.OnContainerList(context, types.ContainerListOptions{All: true}).Return([]types.Container{}, nil)
		c := GetSandbox(context, mockDocker)
		assert.Nil(t, c)
	})

	t.Run("Error in get sandbox container", func(t *testing.T) {
		mockDocker := &mocks.Docker{}
		context := context.Background()

		mockDocker.OnContainerList(context, types.ContainerListOptions{All: true}).Return(containers, nil)
		mockDocker.OnContainerRemove(context, mock.Anything, types.ContainerRemoveOptions{Force: true}).Return(nil)
		err := RemoveSandbox(context, mockDocker, strings.NewReader("y"))
		assert.Nil(t, err)
	})

}

func TestRemoveSandboxWithNoReply(t *testing.T) {
	setupSandbox()
	t.Run("Successfully remove sandbox container", func(t *testing.T) {
		mockDocker := &mocks.Docker{}
		context := context.Background()

		// Verify the attributes
		mockDocker.OnContainerList(context, types.ContainerListOptions{All: true}).Return(containers, nil)
		mockDocker.OnContainerRemove(context, mock.Anything, types.ContainerRemoveOptions{Force: true}).Return(nil)
		err := RemoveSandbox(context, mockDocker, strings.NewReader("n"))
		assert.Nil(t, err)
	})

	t.Run("Successfully remove sandbox container with zero sandbox containers are running", func(t *testing.T) {
		mockDocker := &mocks.Docker{}
		context := context.Background()

		// Verify the attributes
		mockDocker.OnContainerList(context, types.ContainerListOptions{All: true}).Return([]types.Container{}, nil)
		mockDocker.OnContainerRemove(context, mock.Anything, types.ContainerRemoveOptions{Force: true}).Return(nil)
		err := RemoveSandbox(context, mockDocker, strings.NewReader("n"))
		assert.Nil(t, err)
	})

}

func TestPullDockerImage(t *testing.T) {
	t.Run("Successfully pull image", func(t *testing.T) {
		setupSandbox()
		mockDocker := &mocks.Docker{}
		context := context.Background()
		// Verify the attributes
		mockDocker.OnImagePullMatch(context, mock.Anything, types.ImagePullOptions{}).Return(os.Stdin, nil)
		err := PullDockerImage(context, mockDocker, "nginx")
		assert.Nil(t, err)
	})

	t.Run("Error in pull image", func(t *testing.T) {
		setupSandbox()
		mockDocker := &mocks.Docker{}
		context := context.Background()
		// Verify the attributes
		mockDocker.OnImagePullMatch(context, mock.Anything, types.ImagePullOptions{}).Return(os.Stdin, fmt.Errorf("error"))
		err := PullDockerImage(context, mockDocker, "nginx")
		assert.NotNil(t, err)
	})

}

func TestStartContainer(t *testing.T) {
	p1, p2, _ := GetSandboxPorts()

	t.Run("Successfully create a container", func(t *testing.T) {
		setupSandbox()
		mockDocker := &mocks.Docker{}
		context := context.Background()

		// Verify the attributes
		mockDocker.OnContainerCreate(context, &container.Config{
			Env:          Environment,
			Image:        ImageName,
			Tty:          false,
			ExposedPorts: p1,
		}, &container.HostConfig{
			Mounts:       Volumes,
			PortBindings: p2,
			Privileged:   true,
		}, nil, nil, mock.Anything).Return(container.ContainerCreateCreatedBody{
			ID: "Hello",
		}, nil)
		mockDocker.OnContainerStart(context, "Hello", types.ContainerStartOptions{}).Return(nil)
		id, err := StartContainer(context, mockDocker, Volumes, p1, p2, "nginx", ImageName)
		assert.Nil(t, err)
		assert.Greater(t, len(id), 0)
		assert.Equal(t, id, "Hello")
	})

	t.Run("Error in creating container", func(t *testing.T) {
		setupSandbox()
		mockDocker := &mocks.Docker{}
		context := context.Background()

		// Verify the attributes
		mockDocker.OnContainerCreate(context, &container.Config{
			Env:          Environment,
			Image:        ImageName,
			Tty:          false,
			ExposedPorts: p1,
		}, &container.HostConfig{
			Mounts:       Volumes,
			PortBindings: p2,
			Privileged:   true,
		}, nil, nil, mock.Anything).Return(container.ContainerCreateCreatedBody{
			ID: "",
		}, fmt.Errorf("error"))
		mockDocker.OnContainerStart(context, "Hello", types.ContainerStartOptions{}).Return(nil)
		id, err := StartContainer(context, mockDocker, Volumes, p1, p2, "nginx", ImageName)
		assert.NotNil(t, err)
		assert.Equal(t, len(id), 0)
		assert.Equal(t, id, "")
	})

	t.Run("Error in start of a container", func(t *testing.T) {
		setupSandbox()
		mockDocker := &mocks.Docker{}
		context := context.Background()

		// Verify the attributes
		mockDocker.OnContainerCreate(context, &container.Config{
			Env:          Environment,
			Image:        ImageName,
			Tty:          false,
			ExposedPorts: p1,
		}, &container.HostConfig{
			Mounts:       Volumes,
			PortBindings: p2,
			Privileged:   true,
		}, nil, nil, mock.Anything).Return(container.ContainerCreateCreatedBody{
			ID: "Hello",
		}, nil)
		mockDocker.OnContainerStart(context, "Hello", types.ContainerStartOptions{}).Return(fmt.Errorf("error"))
		id, err := StartContainer(context, mockDocker, Volumes, p1, p2, "nginx", ImageName)
		assert.NotNil(t, err)
		assert.Equal(t, len(id), 0)
		assert.Equal(t, id, "")
	})
}

func TestWatchError(t *testing.T) {
	setupSandbox()
	mockDocker := &mocks.Docker{}
	context := context.Background()
	errCh := make(chan error)
	bodyStatus := make(chan container.ContainerWaitOKBody)
	mockDocker.OnContainerWaitMatch(context, mock.Anything, container.WaitConditionNotRunning).Return(bodyStatus, errCh)
	_, err := WatchError(context, mockDocker, "test")
	assert.NotNil(t, err)
}

func TestReadLogs(t *testing.T) {
	setupSandbox()

	t.Run("Successfully read logs", func(t *testing.T) {
		mockDocker := &mocks.Docker{}
		context := context.Background()
		mockDocker.OnContainerLogsMatch(context, mock.Anything, types.ContainerLogsOptions{
			ShowStderr: true,
			ShowStdout: true,
			Timestamps: true,
			Follow:     true,
		}).Return(nil, nil)
		_, err := ReadLogs(context, mockDocker, "test")
		assert.Nil(t, err)
	})

	t.Run("Error in reading logs", func(t *testing.T) {
		mockDocker := &mocks.Docker{}
		context := context.Background()
		mockDocker.OnContainerLogsMatch(context, mock.Anything, types.ContainerLogsOptions{
			ShowStderr: true,
			ShowStdout: true,
			Timestamps: true,
			Follow:     true,
		}).Return(nil, fmt.Errorf("error"))
		_, err := ReadLogs(context, mockDocker, "test")
		assert.NotNil(t, err)
	})
}

func TestWaitForSandbox(t *testing.T) {
	setupSandbox()
	t.Run("Successfully read logs ", func(t *testing.T) {
		reader := bufio.NewScanner(strings.NewReader("hello \n Flyte"))

		check := WaitForSandbox(reader, "Flyte")
		assert.Equal(t, true, check)
	})

	t.Run("Error in reading logs ", func(t *testing.T) {
		reader := bufio.NewScanner(strings.NewReader(""))
		check := WaitForSandbox(reader, "Flyte")
		assert.Equal(t, false, check)
	})
}

func TestDockerClient(t *testing.T) {
	t.Run("Successfully get docker mock client", func(t *testing.T) {
		mockDocker := &mocks.Docker{}
		Client = mockDocker
		cli, err := GetDockerClient()
		assert.Nil(t, err)
		assert.NotNil(t, cli)
	})
	t.Run("Successfully get docker client", func(t *testing.T) {
		Client = nil
		cli, err := GetDockerClient()
		assert.Nil(t, err)
		assert.NotNil(t, cli)
	})

}
