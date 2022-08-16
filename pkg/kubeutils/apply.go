package kubeutils

import (
	"bytes"
	"github.com/jonboulle/clockwork"
	"github.com/pkg/errors"
	kuberrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/cmd/apply"
	kubectlcmdutils "k8s.io/kubectl/pkg/cmd/util"
	kubectlutils "k8s.io/kubectl/pkg/util"
	"time"
)

/**
快速创建 mainfest 中定义的对象使用场景类似 kubectl apply
*/

const (
	DefaultNamespace = "default"
)

type applyHelper struct {
	factory kubectlcmdutils.Factory
}

func NewApplyHelper(restGettr genericclioptions.RESTClientGetter) *applyHelper {
	return &applyHelper{
		factory: kubectlcmdutils.NewFactory(restGettr),
	}
}

// Apply this function use like kubectl apply
func (h *applyHelper) Apply(manifest string) error {
	r := h.factory.NewBuilder().
		Unstructured().
		ContinueOnError().
		NamespaceParam(DefaultNamespace).
		DefaultNamespace().
		Flatten().Stream(bytes.NewBufferString(manifest), "").Do()

	objects, err := r.Infos()
	if err != nil {
		return err
	}

	for _, obj := range objects {
		if err := h.ApplyOneObject(obj); err != nil {
			klog.Error(err)
			return err
		}
	}
	return nil
}

func (*applyHelper) ApplyOneObject(info *resource.Info) error {

	helper := resource.NewHelper(info.Client, info.Mapping)

	modified, err := kubectlutils.GetModifiedConfiguration(info.Object, true, unstructured.UnstructuredJSONScheme)
	if err != nil {
		return err
	}

	if err := info.Get(); err != nil {
		if !kuberrors.IsNotFound(err) {
			return err
		}

		if err := kubectlutils.CreateApplyAnnotation(info.Object, unstructured.UnstructuredJSONScheme); err != nil {
			return errors.Wrapf(err, "create apply annotation")
		}
		if _, err := helper.Create(info.Namespace, true, info.Object); err != nil {
			return errors.Wrapf(err, "create obj")
		}

		return nil
	}

	patcher := apply.Patcher{
		Mapping:           info.Mapping,
		Helper:            helper,
		Overwrite:         true,
		BackOff:           clockwork.NewRealClock(),
		Force:             false,
		CascadingStrategy: metav1.DeletePropagationBackground,
		Timeout:           5 * time.Minute,
		GracePeriod:       1,
		Retries:           5,
	}

	errOut := &bytes.Buffer{}
	if _, _, err := patcher.Patch(info.Object, modified, info.Source, info.Namespace, info.Name, errOut); err != nil {
		return errors.Wrapf(err, errOut.String())
	}
	return nil
}
