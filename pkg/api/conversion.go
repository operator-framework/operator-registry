package api

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/operator-framework/operator-registry/pkg/registry"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func PackageManifestToAPIPackage(manifest *registry.PackageManifest) *Package {
	channels := []*Channel{}
	for _, c := range manifest.Channels {
		channels = append(channels, PackageChannelToAPIChannel(&c))
	}
	return &Package{
		Name:               manifest.PackageName,
		DefaultChannelName: manifest.DefaultChannelName,
		Channels:           channels,
	}
}

func PackageChannelToAPIChannel(channel *registry.PackageChannel) *Channel {
	return &Channel{
		Name:    channel.Name,
		CsvName: channel.CurrentCSVName,
	}
}

func ChannelEntryToAPIChannelEntry(entry *registry.ChannelEntry) *ChannelEntry {
	return &ChannelEntry{
		PackageName: entry.PackageName,
		ChannelName: entry.ChannelName,
		BundleName:  entry.BundleName,
		Replaces:    entry.Replaces,
	}
}

// Bundle strings are appended json objects, we need to split them apart
// e.g. {"my":"obj"}{"csv":"data"}{"crd":"too"}
func BundleStringToObjectStrings(bundleString string) ([]string, error) {
	objs := []string{}
	dec := json.NewDecoder(strings.NewReader(bundleString))

	for dec.More() {
		var m json.RawMessage
		err := dec.Decode(&m)
		if err != nil {
			return nil, err
		}
		objs = append(objs, string(m))
	}
	return objs, nil
}

func BundleStringToAPIBundle(bundleString string, entry *registry.ChannelEntry) (*Bundle, error) {
	objs, err := BundleStringToObjectStrings(bundleString)
	if err != nil {
		return nil, err
	}
	out := &Bundle{
		Object: objs,
	}
	for _, o := range objs {
		dec := yaml.NewYAMLOrJSONDecoder(strings.NewReader(o), 10)
		unst := &unstructured.Unstructured{}
		if err := dec.Decode(unst); err != nil {
			return nil, err
		}
		if unst.GetKind() == "ClusterServiceVersion" {
			out.CsvName = unst.GetName()
			// Here if the environment var REGISTRY_BASE and/or IMAGE_TAG is set, we will update the
			// json string to change the image that will be exposed
			csvJSON, err := updateCSVJSON(unst)
			if err != nil {
				return nil, err
			}
			out.CsvJson = csvJSON
			break
		}
	}
	if out.CsvName == "" {
		return nil, fmt.Errorf("no csv in bundle")
	}
	out.ChannelName = entry.ChannelName
	out.PackageName = entry.PackageName
	return out, nil
}

func updateCSVJSON(unstr *unstructured.Unstructured) (string, error) {
	imageBase := os.Getenv("IMAGE_REGISTRY")
	imageRepo := os.Getenv("IMAGE_REPO")
	imageName := os.Getenv("IMAGE_NAME")
	imageTag := os.Getenv("IMAGE_TAG")
	replaceImages := os.Getenv("REGISTRY_REPLACE_IMAGES")

	imageReplacements := map[string]string{}

	if replaceImages != "" {
		imagesToReplace := strings.Split(replaceImages, ",")
		for _, imageToReplace := range imagesToReplace {
			images := strings.Split(imageToReplace, "=")
			if len(images) != 2 {
				return "", fmt.Errorf("invalid image to replace: %v", imageToReplace)
			}
			imageReplacements[images[0]] = images[1]
		}
	}

	if imageBase == "" && imageTag == "" && imageRepo == "" && imageName == "" && len(imageReplacements) == 0 {
		json, err := unstr.MarshalJSON()
		return string(json), err
	}

	specInterface := unstr.Object["spec"]
	spec, ok := specInterface.(map[string]interface{})
	// If we don't understand just let things continue.
	if !ok {

		fmt.Printf("could not find spec")
		json, err := unstr.MarshalJSON()
		return string(json), err
	}

	installStrategyInterface := spec["install"]
	installStrategy, ok := installStrategyInterface.(map[string]interface{})
	// If we don't understand just let things continue.
	if !ok {
		fmt.Printf("could not find install")
		json, err := unstr.MarshalJSON()
		return string(json), err
	}

	//Assuming deployments for now for the Strategy
	strategySpecInterface := installStrategy["spec"]
	strategySpec, ok := strategySpecInterface.(map[string]interface{})
	// If we don't understand just let things continue.
	if !ok {
		fmt.Printf("could not find spec")
		json, err := unstr.MarshalJSON()
		return string(json), err
	}

	deploymentsInterface := strategySpec["deployments"]
	deployments, ok := deploymentsInterface.([]interface{})
	// If we don't understand just let things continue.
	if !ok {
		fmt.Printf("could not find deployments")
		json, err := unstr.MarshalJSON()
		return string(json), err
	}

	type olmDeploymentSpec struct {
		Name string                 `json:"name"`
		Spec *appsv1.DeploymentSpec `json:"spec"`
	}

	deploymentObjects := []*olmDeploymentSpec{}
	for _, deploymentInterface := range deployments {
		deploymentBytes, err := json.Marshal(deploymentInterface)
		if err != nil {
			fmt.Printf("unable to marshal deploymentsInterface: %v", err)
			json, err := unstr.MarshalJSON()
			return string(json), err
		}

		deploymentSpec := &olmDeploymentSpec{}
		err = json.Unmarshal(deploymentBytes, deploymentSpec)
		if err != nil {
			fmt.Printf("unable to make olmDeploymentSpec: %v", err)
			json, err := unstr.MarshalJSON()
			return string(json), err
		}

		containers := []corev1.Container{}
		for _, container := range deploymentSpec.Spec.Template.Spec.Containers {
			// If we are able to replace the image we should here.
			if i, ok := imageReplacements[container.Image]; ok {
				container.Image = i
				containers = append(containers, container)
				continue
			}
			if imageBase != "" || imageTag != "" || imageRepo != "" || imageName != "" {
				parts := strings.Split(container.Image, "/")
				if len(parts) != 3 {
					fmt.Printf("image is not 3 parts: %v", container.Image)
					json, err := unstr.MarshalJSON()
					return string(json), err
				}
				if imageBase != "" {
					parts[0] = imageBase
				}
				if imageRepo != "" {
					parts[1] = imageRepo
				}
				imageAndTag := strings.Split(parts[2], ":")
				switch len(imageAndTag) {
				case 1:
					if imageName != "" {
						imageAndTag[0] = imageName
					}
					if imageTag != "" {
						imageAndTag = append(imageAndTag, imageTag)
					}
				case 2:
					if imageName != "" {
						imageAndTag[0] = imageName
					}
					if imageTag != "" {
						imageAndTag[1] = imageTag
					}
				}
				parts[2] = strings.Join(imageAndTag, ":")
				container.Image = strings.Join(parts, "/")
				fmt.Printf("container image after update: %v", container.Image)
				containers = append(containers, container)
			}
		}
		deploymentSpec.Spec.Template.Spec.Containers = containers
		deploymentObjects = append(deploymentObjects, deploymentSpec)
	}
	strategySpec["deploymnets"] = deploymentObjects
	installStrategy["spec"] = strategySpec
	spec["install"] = installStrategy
	unstr.Object["spec"] = spec

	json, err := unstr.MarshalJSON()
	return string(json), err
}
