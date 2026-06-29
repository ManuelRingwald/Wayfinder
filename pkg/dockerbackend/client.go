package dockerbackend

// This is the ONLY file that imports the heavy Docker SDK. It is a thin
// translation of the ContainerClient interface onto the Docker Engine API; all
// lifecycle logic lives in backend.go and is tested against a fake. There is no
// unit test here because it requires a running Docker daemon — it is exercised in
// real deployments and kept deliberately minimal.

import (
	"context"
	"strconv"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"
)

// dockerClient implements ContainerClient over the Docker SDK.
type dockerClient struct {
	cli *dockerclient.Client
}

// NewDockerClient connects to the Docker daemon using the standard environment
// (DOCKER_HOST etc.; the unix socket by default) with API-version negotiation, so
// it works across daemon versions. The caller (the orchestrator process) is the
// only component granted access to the socket (ADR 0012 §6).
func NewDockerClient() (ContainerClient, error) {
	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, err
	}
	return &dockerClient{cli: cli}, nil
}

func (d *dockerClient) List(ctx context.Context) ([]ContainerInfo, error) {
	f := filters.NewArgs(filters.Arg("label", labelManaged+"=true"))
	list, err := d.cli.ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		return nil, err
	}
	out := make([]ContainerInfo, 0, len(list))
	for _, c := range list {
		feedID, _ := strconv.ParseInt(c.Labels[labelFeedID], 10, 64)
		out = append(out, ContainerInfo{
			ID:       c.ID,
			FeedID:   feedID,
			Running:  c.State == "running",
			Failed:   c.State == "exited" || c.State == "dead",
			SpecHash: c.Labels[labelSpecHash],
		})
	}
	return out, nil
}

func (d *dockerClient) Create(ctx context.Context, opts CreateOptions) (string, error) {
	resp, err := d.cli.ContainerCreate(ctx,
		&container.Config{Image: opts.Image, Env: opts.Env, Labels: opts.Labels},
		&container.HostConfig{NetworkMode: container.NetworkMode(opts.NetworkMode)},
		nil, nil, opts.Name)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (d *dockerClient) Start(ctx context.Context, id string) error {
	return d.cli.ContainerStart(ctx, id, container.StartOptions{})
}

func (d *dockerClient) Stop(ctx context.Context, id string) error {
	return d.cli.ContainerStop(ctx, id, container.StopOptions{})
}

func (d *dockerClient) Remove(ctx context.Context, id string) error {
	return d.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
}
