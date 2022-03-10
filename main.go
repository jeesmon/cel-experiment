package main

import (
	"fmt"
	"log"
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/interpreter/functions"
	eventspb "github.com/jeesmon/cel-experiment/events"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	"google.golang.org/protobuf/proto"
)

func main() {
	env, err := cel.NewEnv(cel.Lib(experimentLib{}))
	if err != nil {
		log.Fatalf("environment creation error: %v\n", err)
	}

	ast, iss := env.Compile(`hasUID(event,uid)`)
	if iss.Err() != nil {
		log.Fatalln(iss.Err())
	}

	// Serialize AST
	parsed, err := cel.AstToCheckedExpr(ast)
	if err != nil {
		log.Fatalf("AstToParsedExpr() failed: %v", err)
	}

	b, err := proto.Marshal(parsed)
	if err != nil {
		log.Fatalf("proto.Marshal() failed: %v", err)
	}

	// De-Serialized AST
	unmarshalled := exprpb.CheckedExpr{}
	err = proto.Unmarshal(b, &unmarshalled)
	if err != nil {
		log.Fatalf("proto.UnMarshal() failed: %v", err)
	}

	newAst := cel.CheckedExprToAst(&unmarshalled)

	if !proto.Equal(newAst.Expr(), ast.Expr()) {
		log.Fatalf("got expr %v, wanted %v", newAst, ast)
	}

	prg, err := env.Program(newAst)
	if err != nil {
		log.Fatalf("Program creation error: %v\n", err)
	}

	input := map[string]interface{}{
		"uid": "123",
		"event": &eventspb.StudyRevisionEvent{
			Study: &eventspb.DicomStudy{
				StudyInstanceUID: "123",
			},
		},
	}

	out, _, err := prg.Eval(input)
	if err != nil {
		log.Fatalf("Evaluation error: %v\n", err)
	}

	fmt.Printf("%v\n", out)
}

type experimentLib struct{}

func (experimentLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Types(&eventspb.StudyRevisionEvent{}),
		cel.Declarations(
			decls.NewVar("event", decls.NewObjectType("events.StudyRevisionEvent")),
			decls.NewVar("uid", decls.String),
			decls.NewFunction("hasUID",
				decls.NewOverload("has_uid_boolean",
					[]*exprpb.Type{decls.NewObjectType("events.StudyRevisionEvent"), decls.String},
					decls.Bool)),
		),
	}
}

func (experimentLib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{
		cel.Functions(
			&functions.Overload{
				Operator: "has_uid_boolean",
				Binary:   hasUID,
			}),
	}
}

func hasUID(lhs ref.Val, rhs ref.Val) ref.Val {
	v, err := lhs.ConvertToNative(reflect.TypeOf(&eventspb.StudyRevisionEvent{}))
	if err != nil {
		return types.Bool(false)
	}
	event := v.(*eventspb.StudyRevisionEvent)

	uid, ok := rhs.(types.String)
	if !ok {
		return types.ValOrErr(rhs, "unexpected type '%v' passed to shake_hands", uid.Type())
	}

	return types.Bool(types.String(event.Study.StudyInstanceUID) == uid)
}
