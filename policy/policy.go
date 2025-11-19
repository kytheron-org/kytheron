package policy

import (
// "fmt"
// "github.com/theory/jsonpath"
)

// Final structs with resolved references
type Policy struct {
	Name        string
	Sources     []Source
	Evaluations []Evaluation
	Outputs     []Output
	// Destinations []Destination
}

type Source struct {
	Type    string
	Name    string
	Version string
}

type Evaluation struct {
	Type       string
	Name       string
	Inputs     []Source
	Conditions []Condition
	Outputs    []Output
}

type Condition struct {
	Path  string
	Value string
}

type Output struct {
	Type    string
	Name    string
	Version string
}

type Destination struct {
	Type string
	Name string
}

type EvaluationContext struct {
	Policy *Policy // The policy that was evaluated
	logs   []any
}

//type Emit func(p *Policy, data *log.Log) error

func (p *Policy) Process(log any) error {
	//for _, evaluation := range p.Evaluations {
	//	for _, condition := range evaluation.Conditions {
	//		val, err := log.QueryData(condition.Path)
	//		if err != nil {
	//			return err
	//		}
	//
	//		// For now, we'll just handle straight strings
	//		for _, valEntry := range val.(jsonpath.NodeList) {
	//			if valEntry == condition.Value {
	//				// Forward it to the output channel
	//				for _, out := range evaluation.Outputs {
	//					switch out.Type {
	//					case "console":
	//						fmt.Println(log.Data)
	//					}
	//				}
	//			}
	//		}
	//	}
	//}
	return nil
}
