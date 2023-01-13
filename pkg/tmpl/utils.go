package tmpl

import (
	"github.com/pkg/errors"
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"text/template"
)

type Data map[string]interface{}

// Render text template with given `variables` Render-context
func Render(tmpl *template.Template, variables map[string]interface{}) (string, error) {

	var buf strings.Builder

	if err := tmpl.Execute(&buf, variables); err != nil {
		return "", errors.Wrap(err, "Failed to render template")
	}
	return buf.String(), nil
}

// WriteFile
func WriteFiles(content string, filename string, perm fs.FileMode) error {

	dir, _ := path.Split(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil
	}
	return ioutil.WriteFile(filename, []byte(content), perm)

}
