package docker

import (
	"bufio"
	"bytes"
	"github.com/pkg/errors"
	executil "k8s.io/utils/exec"
)

//  考虑 kubernetes 1.24 之后版本已经完全移除了 dockershim ，为了便于后续替换 cri。这里直接通过 shell 命令执行 docker 相关的镜像操作

func ImageLoad(filename string) error {

	executor := executil.New()
	cmd := executor.Command("docker", "image", "load", "--input", filename)
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "error load image file %s", filename)
	}
	return nil
}

func ImagePrune() error {
	executor := executil.New()
	cmd := executor.Command("docker", "image", "prune", "--all", "--force")
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "error prune all image: %s", err.Error())
	}
	return nil
}

func Prune() error {
	executor := executil.New()
	cmd := executor.Command("docker", "container", "prune", "--force")
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "error prune all container: %s", err.Error())
	}
	return nil
}

func Stop(ids ...string) error {
	executor := executil.New()
	args := []string{
		"stop", "--time", "5",
	}
	args = append(args, ids...)

	cmd := executor.Command("docker", args...)
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "error stop container: %s", err.Error())
	}
	return nil
}

func List() ([]string, error) {
	executor := executil.New()
	cmd := executor.Command("docker", "ps", "-q", "--all")

	o, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrapf(err, "error list container: %s", err.Error())
	}

	containerIDs := make([]string, 0)
	sc := bufio.NewScanner(bytes.NewBuffer(o))
	for sc.Scan() {
		containerIDs = append(containerIDs, sc.Text())
	}
	return containerIDs, nil
}
