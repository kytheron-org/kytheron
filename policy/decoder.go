package policy

import (
	"fmt"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty"
)

// Internal structs for HCL decoding (with raw expressions)
type rawPolicy struct {
	Sources      []rawSource      `hcl:"source,block"`
	Evaluations  []rawEvaluation  `hcl:"evaluation,block"`
	Outputs      []rawOutput      `hcl:"output,block"`
	Destinations []rawDestination `hcl:"destination,block"`
	Remain       hcl.Body         `hcl:",remain"`
}

type rawSource struct {
	Type    string   `hcl:"type,label"`
	Name    string   `hcl:"name,label"`
	Version string   `hcl:"version,attr"`
	Remain  hcl.Body `hcl:",remain"`
}

type rawEvaluation struct {
	Type       string         `hcl:"type,label"`
	Name       string         `hcl:"name,label"`
	Inputs     hcl.Expression `hcl:"inputs,attr"`
	Conditions []rawCondition `hcl:"condition,block"`
	Outputs    hcl.Expression `hcl:"outputs,attr"`
	Remain     hcl.Body       `hcl:",remain"`
}

type rawCondition struct {
	Path  string `hcl:"path,attr"`
	Value string `hcl:"value,attr"`
}

type rawOutput struct {
	Type    string   `hcl:"type,label"`
	Name    string   `hcl:"name,label"`
	Version string   `hcl:"version,attr"`
	Remain  hcl.Body `hcl:",remain"`
}

type rawDestination struct {
	Type   string   `hcl:"type,label"`
	Name   string   `hcl:"name,label"`
	Remain hcl.Body `hcl:",remain"`
}

// DecodePolicy parses and decodes an HCL file into a Policy struct
func Decode(filename string, content []byte) (*Policy, error) {
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL(content, filename)
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse error: %s", diags.Error())
	}

	// First pass: decode into raw structs
	var raw rawPolicy
	diags = gohcl.DecodeBody(file.Body, nil, &raw)
	if diags.HasErrors() {
		return nil, fmt.Errorf("decode error: %s", diags.Error())
	}

	// Build evaluation context with all resources
	evalCtx := buildEvalContext(&raw)

	// Second pass: resolve references and build final policy
	policy := &Policy{
		Name:        filename,
		Sources:     make([]Source, len(raw.Sources)),
		Evaluations: make([]Evaluation, len(raw.Evaluations)),
		Outputs:     make([]Output, len(raw.Outputs)),
	}

	// Convert sources
	for i, rs := range raw.Sources {
		policy.Sources[i] = Source{
			Type: rs.Type,
			Name: rs.Name,
		}
	}

	// Convert outputs (resolve evaluation and destination references)
	for i, ro := range raw.Outputs {
		output := Output{
			Type: ro.Type,
			Name: ro.Name,
		}
		policy.Outputs[i] = output
	}

	// Convert evaluations (resolve input references)
	for i, re := range raw.Evaluations {
		eval := Evaluation{
			Type:       re.Type,
			Name:       re.Name,
			Conditions: make([]Condition, len(re.Conditions)),
		}

		// Resolve inputs
		if re.Inputs != nil {
			inputs, err := resolveSourceReferences(re.Inputs, evalCtx, &raw)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve inputs for evaluation %s.%s: %w", re.Type, re.Name, err)
			}
			eval.Inputs = inputs
		}

		// Copy conditions
		for j, rc := range re.Conditions {
			eval.Conditions[j] = Condition{
				Path:  rc.Path,
				Value: rc.Value,
			}
		}

		if re.Outputs != nil {
			fmt.Println("resolving outputs")
			outputs, err := resolveOutputReferences(re.Outputs, evalCtx, &raw)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve outputs for evaluation %s.%s: %w", re.Type, re.Name, err)
			}
			eval.Outputs = outputs
		}

		policy.Evaluations[i] = eval
	}

	return policy, nil
}

// buildEvalContext creates an HCL evaluation context with all resources
func buildEvalContext(raw *rawPolicy) *hcl.EvalContext {
	ctx := &hcl.EvalContext{
		Variables: make(map[string]cty.Value),
	}

	// Add sources
	sourceMap := make(map[string]cty.Value)
	for _, s := range raw.Sources {
		if _, ok := sourceMap[s.Type]; !ok {
			sourceMap[s.Type] = cty.ObjectVal(make(map[string]cty.Value))
		}
		typeMap := sourceMap[s.Type].AsValueMap()
		if typeMap == nil {
			typeMap = make(map[string]cty.Value)
		}
		typeMap[s.Name] = cty.StringVal(fmt.Sprintf("source.%s.%s", s.Type, s.Name))
		sourceMap[s.Type] = cty.ObjectVal(typeMap)
	}
	ctx.Variables["source"] = cty.ObjectVal(sourceMap)

	// Add evaluations
	evalMap := make(map[string]cty.Value)
	for _, e := range raw.Evaluations {
		if _, ok := evalMap[e.Type]; !ok {
			evalMap[e.Type] = cty.ObjectVal(make(map[string]cty.Value))
		}
		typeMap := evalMap[e.Type].AsValueMap()
		if typeMap == nil {
			typeMap = make(map[string]cty.Value)
		}
		typeMap[e.Name] = cty.StringVal(fmt.Sprintf("evaluation.%s.%s", e.Type, e.Name))
		evalMap[e.Type] = cty.ObjectVal(typeMap)
	}
	ctx.Variables["evaluation"] = cty.ObjectVal(evalMap)

	outputMap := make(map[string]cty.Value)
	for _, s := range raw.Outputs {
		if _, ok := outputMap[s.Type]; !ok {
			outputMap[s.Type] = cty.ObjectVal(make(map[string]cty.Value))
		}
		typeMap := outputMap[s.Type].AsValueMap()
		if typeMap == nil {
			typeMap = make(map[string]cty.Value)
		}
		typeMap[s.Name] = cty.StringVal(fmt.Sprintf("output.%s.%s", s.Type, s.Name))
		outputMap[s.Type] = cty.ObjectVal(typeMap)
	}
	ctx.Variables["output"] = cty.ObjectVal(outputMap)

	return ctx
}

// resolveSourceReferences resolves source references from an expression
func resolveSourceReferences(expr hcl.Expression, ctx *hcl.EvalContext, raw *rawPolicy) ([]Source, error) {
	val, diags := expr.Value(ctx)
	if diags.HasErrors() {
		return nil, fmt.Errorf("evaluation error: %s", diags.Error())
	}

	var sources []Source
	if val.Type().IsListType() || val.Type().IsTupleType() {
		for _, item := range val.AsValueSlice() {
			if !item.Type().Equals(cty.String) {
				continue
			}
			ref := item.AsString()
			source, err := findSourceByRef(ref, raw)
			if err != nil {
				return nil, err
			}
			sources = append(sources, source)
		}
	}

	return sources, nil
}

// resolveEvaluationReference resolves an evaluation reference
func resolveEvaluationReference(expr hcl.Expression, ctx *hcl.EvalContext, policy *Policy) (*Evaluation, error) {
	val, diags := expr.Value(ctx)
	if diags.HasErrors() {
		return nil, fmt.Errorf("evaluation error: %s", diags.Error())
	}

	if !val.Type().Equals(cty.String) {
		return nil, fmt.Errorf("expected string reference")
	}

	ref := val.AsString()
	return findEvaluationByRef(ref, policy)
}

// resolveOutputReference resolves a destination reference
func resolveOutputReferences(expr hcl.Expression, ctx *hcl.EvalContext, raw *rawPolicy) ([]Output, error) {
	val, diags := expr.Value(ctx)
	if diags.HasErrors() {
		return nil, fmt.Errorf("evaluation error: %s", diags.Error())
	}

	var outputs []Output
	if val.Type().IsListType() || val.Type().IsTupleType() {
		for _, item := range val.AsValueSlice() {
			if !item.Type().Equals(cty.String) {
				continue
			}
			ref := item.AsString()
			output, err := findOutputByRef(ref, raw)
			if err != nil {
				return nil, err
			}
			outputs = append(outputs, output)
		}
	} else {
		fmt.Println("Not processing value altogether")
	}

	return outputs, nil
}

// Helper functions to find resources by reference string
func findSourceByRef(ref string, raw *rawPolicy) (Source, error) {
	for _, s := range raw.Sources {
		if ref == fmt.Sprintf("source.%s.%s", s.Type, s.Name) {
			return Source{Type: s.Type, Name: s.Name}, nil
		}
	}
	return Source{}, fmt.Errorf("source not found: %s", ref)
}

func findEvaluationByRef(ref string, policy *Policy) (*Evaluation, error) {
	for i := range policy.Evaluations {
		if ref == fmt.Sprintf("evaluation.%s.%s", policy.Evaluations[i].Type, policy.Evaluations[i].Name) {
			return &policy.Evaluations[i], nil
		}
	}
	return nil, fmt.Errorf("evaluation not found: %s", ref)
}

func findOutputByRef(ref string, policy *rawPolicy) (Output, error) {
	for _, s := range policy.Outputs {
		if ref == fmt.Sprintf("output.%s.%s", s.Type, s.Name) {
			return Output{Type: s.Type, Name: s.Name}, nil
		}
	}
	return Output{}, fmt.Errorf("output not found: %s", ref)
}
