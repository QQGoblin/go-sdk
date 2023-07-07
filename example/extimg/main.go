package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/QQGoblin/go-sdk/pkg/tarutil"
	"github.com/pkg/errors"
	"os"
	"path/filepath"
	"strings"
)

var (
	base string
)

const (
	manifestFilename = "manifest.json"
)

func init() {
	flag.StringVar(&base, "dir", "/opt/rcos_docker/docker_image", "tar file base dir")

}

func main() {
	flag.Parse()

	if err := filepath.Walk(base, pasreTarFile); err != nil {
		panic(err)
	}

}

type imageManifest struct {
	Config   string   `json:"Config"`
	RepoTags []string `json:"RepoTags"`
	Layers   []string `json:"Layers"`
}

func pasreTarFile(fulltarname string, info os.FileInfo, err error) error {

	if info.IsDir() || !strings.HasSuffix(info.Name(), ".tar") {
		return nil
	}

	repMessage, err := tarutil.ExtractedByName(fulltarname, manifestFilename)
	if err != nil {
		return errors.Wrapf(err, "extracted repositories file failed for %s", fulltarname)
	}

	var manifest []imageManifest

	if json.Unmarshal(repMessage, &manifest); err != nil {
		return err
	}

	for _, m := range manifest {
		fmt.Println(m.RepoTags[0])
	}
	return nil
}
