package common

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/kubevirt/terraform-provider-kubevirt/kubevirt/utils"
	"github.com/kubevirt/terraform-provider-kubevirt/kubevirt/utils/patch"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func metadataFields(objectName string) map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"annotations": {
			Type:         schema.TypeMap,
			Description:  fmt.Sprintf("An unstructured key value map stored with the %s that may be used to store arbitrary metadata. More info: http://kubernetes.io/docs/user-guide/annotations", objectName),
			Optional:     true,
			Elem:         &schema.Schema{Type: schema.TypeString},
			ValidateFunc: validateAnnotations,
		},
		"generation": {
			Type:        schema.TypeInt,
			Description: "A sequence number representing a specific generation of the desired state.",
			Computed:    true,
		},
		"labels": {
			Type:         schema.TypeMap,
			Description:  fmt.Sprintf("Map of string keys and values that can be used to organize and categorize (scope and select) the %s. May match selectors of replication controllers and services. More info: http://kubernetes.io/docs/user-guide/labels", objectName),
			Optional:     true,
			Elem:         &schema.Schema{Type: schema.TypeString},
			ValidateFunc: validateLabels,
		},
		"name": {
			Type:         schema.TypeString,
			Description:  fmt.Sprintf("Name of the %s, must be unique. Cannot be updated. More info: http://kubernetes.io/docs/user-guide/identifiers#names", objectName),
			Optional:     true,
			ForceNew:     true,
			Computed:     true,
			ValidateFunc: validateName,
		},
		"resource_version": {
			Type:        schema.TypeString,
			Description: fmt.Sprintf("An opaque value that represents the internal version of this %s that can be used by clients to determine when %s has changed. Read more: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#concurrency-control-and-consistency", objectName, objectName),
			Computed:    true,
		},
		"self_link": {
			Type:        schema.TypeString,
			Description: fmt.Sprintf("A URL representing this %s.", objectName),
			Computed:    true,
		},
		"uid": {
			Type:        schema.TypeString,
			Description: fmt.Sprintf("The unique in time and space value for this %s. More info: http://kubernetes.io/docs/user-guide/identifiers#uids", objectName),
			Computed:    true,
		},
	}
}

func metadataSchema(objectName string, generatableName bool) *schema.Schema {
	fields := metadataFields(objectName)

	if generatableName {
		fields["generate_name"] = &schema.Schema{
			Type:          schema.TypeString,
			Description:   "Prefix, used by the server, to generate a unique name ONLY IF the `name` field has not been provided. This value will also be combined with a unique suffix. Read more: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#idempotency",
			Optional:      true,
			ForceNew:      true,
			ValidateFunc:  validateGenerateName,
			ConflictsWith: []string{"metadata.0.name"},
		}
		fields["name"].ConflictsWith = []string{"metadata.0.generate_name"}
	}

	return &schema.Schema{
		Type:        schema.TypeList,
		Description: fmt.Sprintf("Standard %s's metadata. More info: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#metadata", objectName),
		Required:    true,
		MaxItems:    1,
		Elem: &schema.Resource{
			Schema: fields,
		},
	}

}

func NamespacedMetadataSchema(objectName string, generatableName bool) *schema.Schema {
	return namespacedMetadataSchemaIsTemplate(objectName, generatableName, false)
}

func namespacedMetadataSchemaIsTemplate(objectName string, generatableName, isTemplate bool) *schema.Schema {
	fields := metadataFields(objectName)
	fields["namespace"] = &schema.Schema{
		Type:        schema.TypeString,
		Description: fmt.Sprintf("Namespace defines the space within which name of the %s must be unique.", objectName),
		Optional:    true,
		ForceNew:    true,
		Default:     utils.ConditionalDefault(!isTemplate, "default"),
	}
	if generatableName {
		fields["generate_name"] = &schema.Schema{
			Type:          schema.TypeString,
			Description:   "Prefix, used by the server, to generate a unique name ONLY IF the `name` field has not been provided. This value will also be combined with a unique suffix. Read more: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#idempotency",
			Optional:      true,
			ForceNew:      true,
			ValidateFunc:  validateGenerateName,
			ConflictsWith: []string{"metadata.name"},
		}
		fields["name"].ConflictsWith = []string{"metadata.generate_name"}
	}

	return &schema.Schema{
		Type:        schema.TypeList,
		Description: fmt.Sprintf("Standard %s's metadata. More info: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#metadata", objectName),
		Required:    true,
		MaxItems:    1,
		Elem: &schema.Resource{
			Schema: fields,
		},
	}
}

func BuildId(meta metav1.ObjectMeta) string {
	return meta.Namespace + "/" + meta.Name
}

func ExpandMetadata(in []interface{}) metav1.ObjectMeta {
	meta := metav1.ObjectMeta{}
	if len(in) < 1 {
		return meta
	}
	m := in[0].(map[string]interface{})

	if v, ok := m["annotations"].(map[string]interface{}); ok && len(v) > 0 {
		meta.Annotations = utils.ExpandStringMap(m["annotations"].(map[string]interface{}))
	}

	if v, ok := m["labels"].(map[string]interface{}); ok && len(v) > 0 {
		meta.Labels = utils.ExpandStringMap(m["labels"].(map[string]interface{}))
	}

	if v, ok := m["generate_name"]; ok {
		meta.GenerateName = v.(string)
	}
	if v, ok := m["name"]; ok {
		meta.Name = v.(string)
	}
	if v, ok := m["namespace"]; ok {
		meta.Namespace = v.(string)
	}

	return meta
}

func FlattenMetadata(meta metav1.ObjectMeta, d *schema.ResourceData, metaPrefix ...string) []interface{} {
	m := make(map[string]interface{})
	prefix := ""
	if len(metaPrefix) > 0 {
		prefix = metaPrefix[0]
	}
	configAnnotations := d.Get(prefix + "metadata.0.annotations").(map[string]interface{})
	m["annotations"] = removeInternalKeys(meta.Annotations, configAnnotations)
	if meta.GenerateName != "" {
		m["generate_name"] = meta.GenerateName
	}
	configLabels := d.Get(prefix + "metadata.0.labels").(map[string]interface{})
	m["labels"] = removeInternalKeys(meta.Labels, configLabels)
	m["name"] = meta.Name
	m["resource_version"] = meta.ResourceVersion
	m["self_link"] = meta.SelfLink
	m["uid"] = fmt.Sprintf("%v", meta.UID)
	m["generation"] = meta.Generation

	if meta.Namespace != "" {
		m["namespace"] = meta.Namespace
	}

	return []interface{}{m}
}

func PatchMetadata(keyPrefix, pathPrefix string, d *schema.ResourceData) patch.PatchOperations {
	ops := make([]patch.PatchOperation, 0, 0)
	if d.HasChange(keyPrefix + "annotations") {
		oldV, newV := d.GetChange(keyPrefix + "annotations")
		diffOps := patch.DiffStringMap(pathPrefix+"annotations", oldV.(map[string]interface{}), newV.(map[string]interface{}))
		ops = append(ops, diffOps...)
	}
	if d.HasChange(keyPrefix + "labels") {
		oldV, newV := d.GetChange(keyPrefix + "labels")
		diffOps := patch.DiffStringMap(pathPrefix+"labels", oldV.(map[string]interface{}), newV.(map[string]interface{}))
		ops = append(ops, diffOps...)
	}
	return ops
}

func removeInternalKeys(m map[string]string, d map[string]interface{}) map[string]string {
	for k := range m {
		if isInternalKey(k) && !isKeyInMap(k, d) {
			delete(m, k)
		}
	}
	return m
}

func isKeyInMap(key string, d map[string]interface{}) bool {
	if d == nil {
		return false
	}
	for k := range d {
		if k == key {
			return true
		}
	}
	return false
}

func isInternalKey(annotationKey string) bool {
	u, err := url.Parse("//" + annotationKey)
	if err == nil && strings.HasSuffix(u.Hostname(), "kubernetes.io") {
		return true
	}

	// Specific to DaemonSet annotations, generated & controlled by the server.
	if strings.Contains(annotationKey, "deprecated.daemonset.template.generation") {
		return true
	}

	return false
}