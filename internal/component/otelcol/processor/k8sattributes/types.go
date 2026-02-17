package k8sattributes

type FieldExtractConfig struct {
	TagName  string `alloy:"tag_name,attr,optional"`
	Key      string `alloy:"key,attr,optional"`
	KeyRegex string `alloy:"key_regex,attr,optional"`
	From     string `alloy:"from,attr,optional"`
}

func (args FieldExtractConfig) convert() map[string]any {
	return map[string]any{
		"tag_name":  args.TagName,
		"key":       args.Key,
		"key_regex": args.KeyRegex,
		"from":      args.From,
	}
}

type ExtractConfig struct {
	Metadata                     []string             `alloy:"metadata,attr,optional"`
	Annotations                  []FieldExtractConfig `alloy:"annotation,block,optional"`
	Labels                       []FieldExtractConfig `alloy:"label,block,optional"`
	OtelAnnotations              bool                 `alloy:"otel_annotations,attr,optional"`
	DeploymentNameFromReplicaSet bool                 `alloy:"deployment_name_from_replicaset,attr,optional"`
}

func (args ExtractConfig) convert() map[string]any {
	annotations := make([]any, 0, len(args.Annotations))

	for _, annotation := range args.Annotations {
		annotations = append(annotations, annotation.convert())
	}

	labels := make([]any, 0, len(args.Labels))
	for _, label := range args.Labels {
		labels = append(labels, label.convert())
	}

	return map[string]any{
		"metadata":                        args.Metadata,
		"annotations":                     annotations,
		"labels":                          labels,
		"otel_annotations":                args.OtelAnnotations,
		"deployment_name_from_replicaset": args.DeploymentNameFromReplicaSet,
	}
}

type FieldFilterConfig struct {
	Key   string `alloy:"key,attr"`
	Value string `alloy:"value,attr"`
	Op    string `alloy:"op,attr,optional"`
}

func (args FieldFilterConfig) convert() map[string]any {
	return map[string]any{
		"key":   args.Key,
		"value": args.Value,
		"op":    args.Op,
	}
}

type FilterConfig struct {
	Node      string              `alloy:"node,attr,optional"`
	Namespace string              `alloy:"namespace,attr,optional"`
	Fields    []FieldFilterConfig `alloy:"field,block,optional"`
	Labels    []FieldFilterConfig `alloy:"label,block,optional"`
}

func (args FilterConfig) convert() map[string]any {
	result := make(map[string]any)

	if args.Node != "" {
		result["node"] = args.Node
	}

	if args.Namespace != "" {
		result["namespace"] = args.Namespace
	}

	fields := make([]any, 0, len(args.Fields))
	for _, field := range args.Fields {
		fields = append(fields, field.convert())
	}

	if len(fields) > 0 {
		result["fields"] = fields
	}

	labels := make([]any, 0, len(args.Labels))
	for _, label := range args.Labels {
		labels = append(labels, label.convert())
	}

	if len(labels) > 0 {
		result["labels"] = labels
	}

	return result
}

type PodAssociation struct {
	Sources []PodAssociationSource `alloy:"source,block"`
}

func (args PodAssociation) convert() []map[string]any {
	result := make([]map[string]any, 0, len(args.Sources))

	for _, source := range args.Sources {
		result = append(result, source.convert())
	}

	return result
}

type PodAssociationSource struct {
	From string `alloy:"from,attr"`
	Name string `alloy:"name,attr,optional"`
}

func (args PodAssociationSource) convert() map[string]any {
	return map[string]any{
		"from": args.From,
		"name": args.Name,
	}
}

type PodAssociationSlice []PodAssociation

func (args PodAssociationSlice) convert() []map[string]any {
	result := make([]map[string]any, 0, len(args))

	for _, podAssociation := range args {
		result = append(result, map[string]any{
			"sources": podAssociation.convert(),
		})
	}

	return result
}

type ExcludeConfig struct {
	Pods []ExcludePodConfig `alloy:"pod,block,optional"`
}

type ExcludePodConfig struct {
	Name string `alloy:"name,attr"`
}

func (args ExcludePodConfig) convert() map[string]any {
	return map[string]any{
		"name": args.Name,
	}
}

func (args ExcludeConfig) convert() map[string]any {
	result := make(map[string]any)

	pods := make([]any, 0, len(args.Pods))
	for _, pod := range args.Pods {
		pods = append(pods, pod.convert())
	}

	result["pods"] = pods

	return result
}
