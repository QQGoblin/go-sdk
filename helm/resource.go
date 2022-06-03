package helm

import (
	"bytes"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	kubectlutils "k8s.io/kubectl/pkg/cmd/util"
	"strings"
)

func IsReady(release, namespace string) bool {
	// 判断 helm 部署的 pod、deployment、statefulset 等资源是否处于 Ready 状态
	isReady := false
	return isReady
}

func Images(chartPath string) ([]string, error) {
	// 提取 helm 包中的镜像信息

	chart, err := loader.Load(chartPath)
	if err != nil {
		klog.Errorf("load chart file %s failed: %v", chartPath, err)
		return nil, err
	}

	// TODO: 外部传入的 Values 参数
	vals := make(map[string]interface{})
	options := chartutil.ReleaseOptions{
		Name:      "release-name",
		Namespace: "default",
		Revision:  1,
		IsUpgrade: false,
		IsInstall: true,
	}
	valuesToRender, err := chartutil.ToRenderValues(chart, vals, options, nil)
	if err != nil {
		klog.Errorf("reader charts values failed: %v", err)
		return nil, err
	}
	resource, err := engine.Render(chart, valuesToRender)
	if err != nil {
		klog.Errorf("reader charts failed: %v", err)
		return nil, err
	}

	getter := genericclioptions.NewConfigFlags(true)
	builder := kubectlutils.NewFactory(getter).NewBuilder().
		ContinueOnError().Flatten().
		Local().
		Unstructured()

	workloads := sets.NewString("Deployment", "Pod", "StatefulSet", "DaemonSet", "Job", "CronJob")
	images := sets.NewString()

	for f, r := range resource {
		if !(strings.HasSuffix(f, "yaml") || strings.HasSuffix(f, "yml")) {
			continue
		}
		infos, err := builder.Stream(bytes.NewBufferString(r), "").Do().Infos()
		if err != nil {
			klog.Errorf("get runtime object from %s failed: %v", f, err)
			return nil, err
		}
		for _, info := range infos {
			kind := info.Object.GetObjectKind().GroupVersionKind().Kind
			if !workloads.Has(kind) {
				continue
			}
			unstructured, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(info.Object)
			parseImages(unstructured, &images)
		}
	}
	return images.List(), nil
}

func parseImages(unstructured interface{}, images *sets.String) {

	if _, isMap := unstructured.(map[string]interface{}); isMap {
		for k, v := range unstructured.(map[string]interface{}) {
			if k == "containers" || k == "initContainers" {
				containers := v.([]interface{})
				for _, unc := range containers {
					c := unc.(map[string]interface{})
					if i, isOk := c["image"]; isOk {
						images.Insert(i.(string))
					}
				}
				return
			}
			parseImages(v, images)
		}
	}
}
