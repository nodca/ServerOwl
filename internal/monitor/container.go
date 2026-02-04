package monitor

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

var (
	dockerClient     *client.Client
	dockerClientOnce sync.Once
)

func getDockerClient() (*client.Client, error) {
	var err error
	dockerClientOnce.Do(func() {
		dockerClient, err = client.NewClientWithOpts(client.FromEnv)
	})
	return dockerClient, err
}

type ContainerCheckResult struct {
	Name    string
	ID      string
	Running bool
	Status  string // "running", "exited", "paused" 等
	Error   string
}

func CheckContainer(containerID string) ContainerCheckResult {
	cli, err := getDockerClient()
	if err != nil {
		return ContainerCheckResult{
			ID:    containerID,
			Error: err.Error(),
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	container, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return ContainerCheckResult{
			ID:      containerID,
			Error:   err.Error(),
			Running: false,
		}
	} else {
		return ContainerCheckResult{
			Name:    container.Name,
			ID:      containerID,
			Running: container.State.Running,
			Status:  container.State.Status,
		}
	}
}

// 重启容器
func RestartContainer(containerID string, timeout time.Duration) error {
	cli, err := getDockerClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return cli.ContainerRestart(ctx, containerID, container.StopOptions{})
}

// 获取容器日志
func GetContainerLogs(containerID string, lines int) (string, error) {
	cli, err := getDockerClient()
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       fmt.Sprintf("%d", lines), //文件末尾lines行
	}

	reader, err := cli.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	// Docker 日志有 8 字节头部，需要用 stdcopy 解析
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	_, err = stdcopy.StdCopy(stdout, stderr, reader)
	if err != nil {
		return "", err
	}

	// 合并 stdout 和 stderr
	result := stdout.String()
	if stderr.Len() > 0 {
		result += "\n[STDERR]\n" + stderr.String()
	}

	return result, nil
}
