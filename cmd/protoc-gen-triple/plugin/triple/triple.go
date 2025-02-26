/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package dubbo

import (
	"fmt"
	"strconv"
	"strings"
)

import (
	pb "github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/golang/protobuf/protoc-gen-go/generator"
)

// generatedCodeVersion indicates a version of the generated codes.
// It is incremented whenever an incompatibility between the generated codes and
// the grpc package is introduced; the generated codes references
// a constant, grpc.SupportPackageIsVersionN (where N is generatedCodeVersion).
const generatedCodeVersion = 4

// Paths for packages used by codes generated in this file,
// relative to the import_prefix of the generator.Generator.
const (
	contextPkgPath = "context"
	grpcPkgPath    = "google.golang.org/grpc"
	codePkgPath    = "google.golang.org/grpc/codes"
	statusPkgPath  = "google.golang.org/grpc/status"
)

func init() {
	generator.RegisterPlugin(new(triple))
}

// triple is an implementation of the Go protocol buffer compiler's
// plugin architecture.  It generates bindings for gRPC-triple support.
type triple struct {
	gen *generator.Generator
}

// Name returns the name of this plugin, "grpc".
func (g *triple) Name() string {
	return "triple"
}

// The names for packages imported in the generated codes.
// They may vary from the final path component of the import path
// if the name is used by other packages.
var (
	contextPkg string
	grpcPkg    string
)

// Init initializes the plugin.
func (g *triple) Init(gen *generator.Generator) {
	g.gen = gen
}

// Given a type name defined in a .proto, return its object.
// Also record that we're using it, to guarantee the associated import.
func (g *triple) objectNamed(name string) generator.Object {
	g.gen.RecordTypeUse(name)
	return g.gen.ObjectNamed(name)
}

// Given a type name defined in a .proto, return its name as we will print it.
func (g *triple) typeName(str string) string {
	return g.gen.TypeName(g.objectNamed(str))
}

// P forwards to g.gen.P.
func (g *triple) P(args ...interface{}) { g.gen.P(args...) }

// Generate generates codes for the services in the given file.
// be consistent with grpc plugin
func (g *triple) Generate(file *generator.FileDescriptor) {
	if len(file.FileDescriptorProto.Service) == 0 {
		return
	}

	contextPkg = string(g.gen.AddImport(contextPkgPath))
	grpcPkg = string(g.gen.AddImport(grpcPkgPath))

	for i, service := range file.FileDescriptorProto.Service {
		g.generateService(file, service, i)
	}
}

// GenerateImports generates the import declaration for this file.
func (g *triple) GenerateImports(file *generator.FileDescriptor) {
	g.P("import (")
	g.P(`dgrpc "dubbo.apache.org/dubbo-go/v3/protocol/dubbo3"`)
	g.P(`"dubbo.apache.org/dubbo-go/v3/protocol/invocation"`)
	g.P(`"dubbo.apache.org/dubbo-go/v3/protocol"`)
	g.P(`dubbo3 "github.com/dubbogo/triple/pkg/triple"`)
	g.P(`tripleConstant "github.com/dubbogo/triple/pkg/common/constant"`)
	g.P(`"github.com/dubbogo/triple/pkg/common"`)
	g.P(` ) `)
}

func unexport(s string) string { return strings.ToLower(s[:1]) + s[1:] }

// deprecationComment is the standard comment added to deprecated
// messages, fields, enums, and enum values.
var deprecationComment = "// Deprecated: Do not use."

// generateService generates all the codes for the named service.
func (g *triple) generateService(file *generator.FileDescriptor, service *pb.ServiceDescriptorProto, index int) {
	path := fmt.Sprintf("6,%d", index) // 6 means service.
	origServName := service.GetName()
	fullServName := origServName
	if pkg := file.GetPackage(); pkg != "" {
		fullServName = pkg + "." + fullServName
	}
	servName := generator.CamelCase(origServName)
	lowerServName := strings.ToLower(servName)
	lowerFrontServeName := strings.ToLower(servName[:1]) + servName[1:]
	g.P(fmt.Sprintf("type %sDubbo3Client struct {", lowerServName))
	g.P(fmt.Sprintf("cc *dubbo3.TripleConn"))
	g.P("}")

	g.P(fmt.Sprintf("func New%sDubbo3Client (cc *dubbo3.TripleConn) %sClient {", servName, servName))
	g.P(fmt.Sprintf("return &%sDubbo3Client{cc}", lowerServName))
	g.P(fmt.Sprintf("}"))

	for _, method := range service.Method {
		inputTypeNames := strings.Split(method.GetInputType(), ".")
		inputTypeName := inputTypeNames[len(inputTypeNames)-1]
		outputTypeNames := strings.Split(method.GetOutputType(), ".")
		outputTypeName := outputTypeNames[len(outputTypeNames)-1]
		if method.GetServerStreaming() || method.GetClientStreaming() {
			//now we only support two way streaming
			g.P(fmt.Sprintf("func (c *%sDubbo3Client) %s(ctx %s.Context,opts ...grpc.CallOption) (%s, error) {",
				lowerServName, method.GetName(), contextPkg, servName+"_"+method.GetName()+"Client"))
			g.P(fmt.Sprintf("interfaceKey := ctx.Value(tripleConstant.InterfaceKey).(string)"))
			g.P(fmt.Sprintf("stream, err := c.cc.NewStream(ctx, \"/\" + interfaceKey + \"/%s\", opts...)", method.GetName()))
			g.P("if err != nil {")
			g.P("return nil, err")
			g.P("}")
			g.P(fmt.Sprintf("x := &%s%sClient{stream}", lowerFrontServeName, method.GetName()))
			g.P("return x, nil")
			g.P("}")
			continue
		}
		// unary rpc method client
		g.P(fmt.Sprintf("func (c *%sDubbo3Client) %s(ctx %s.Context, in *%s, opts ...grpc.CallOption) (*%s, common.ErrorWithAttachment) {",
			lowerServName, method.GetName(), contextPkg, inputTypeName, outputTypeName))
		g.P(fmt.Sprintf("out := new(%s)", outputTypeName))
		g.P(fmt.Sprintf("interfaceKey := ctx.Value(tripleConstant.InterfaceKey).(string)"))
		g.P(fmt.Sprintf("return out, c.cc.Invoke(ctx, \"/\" + interfaceKey + \"/%s\", in, out)", method.GetName()))
		g.P("}")
	}

	deprecated := service.GetOptions().GetDeprecated()

	g.P()
	g.P(fmt.Sprintf(`// %sClientImpl is the client API for %s service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.`, servName, servName))

	// Client interface.
	if deprecated {
		g.P("//")
		g.P(deprecationComment)
	}

	g.P("type ", servName, "Client interface {")
	for i, method := range service.Method {
		g.gen.PrintComments(fmt.Sprintf("%s,2,%d", path, i)) // 2 means method in a service.
		if method.GetOptions().GetDeprecated() {
			g.P("//")
			g.P(deprecationComment)
		}
		g.P(g.generateTripleClientSignature(servName, method))
	}
	g.P("}")
	g.P()

	dubboSrvName := servName + "ClientImpl"
	g.P("type ", dubboSrvName, " struct {")
	for i, method := range service.Method {
		g.gen.PrintComments(fmt.Sprintf("%s,2,%d", path, i)) // 2 means method in a service.
		if method.GetOptions().GetDeprecated() {
			g.P("//")
			g.P(deprecationComment)
		}
		g.P(g.generateClientSignature(servName, method))
	}
	g.P("}")
	g.P()

	// NewClient factory.
	if deprecated {
		g.P(deprecationComment)
	}

	var methodIndex, streamIndex int
	serviceDescVar := "_" + servName + "_serviceDesc"
	// Client method implementations.
	for _, method := range service.Method {
		var descExpr string
		if !method.GetServerStreaming() && !method.GetClientStreaming() {
			// Unary RPC method
			descExpr = fmt.Sprintf("&%s.Methods[%d]", serviceDescVar, methodIndex)
			methodIndex++
		} else {
			// Streaming RPC method
			descExpr = fmt.Sprintf("&%s.Streams[%d]", serviceDescVar, streamIndex)
			streamIndex++
		}
		g.generateClientMethod(servName, fullServName, serviceDescVar, method, descExpr)
	}

	// add Reference method
	//func (u *GrpcGreeterImpl) Reference() string {
	//	return "GrpcGreeterImpl"
	//}
	g.P("func (c *", dubboSrvName, ") ", " Reference() string ", "{")
	g.P(`return "`, unexport(servName), `Impl"`)
	g.P("}")
	g.P()

	// add GetDubboStub method
	// func (u *GrpcGreeterImpl) GetDubboStub(cc *dubbo3.TripleConn) GreeterClient {
	//	return NewGreeterClient(cc)
	//}
	g.P("func (c *", dubboSrvName, ") ", " GetDubboStub(cc *dubbo3.TripleConn) ", servName, "Client {")
	g.P(`return New`, servName, `Dubbo3Client(cc)`)
	g.P("}")
	g.P()

	// Server interface.
	serverType := servName + "Server"
	g.P("// ", serverType, " is the server API for ", servName, " service.")
	if deprecated {
		g.P("//")
		g.P(deprecationComment)
	}
	g.P("type ", serverType, " interface {")
	for i, method := range service.Method {
		g.gen.PrintComments(fmt.Sprintf("%s,2,%d", path, i)) // 2 means method in a service.
		if method.GetOptions().GetDeprecated() {
			g.P("//")
			g.P(deprecationComment)
		}
		g.P(g.generateTripleServerSignature(servName, method))
	}
	g.P("}")
	g.P()

	tripleServerBaseType := servName + "ProviderBase"
	g.P("type ", tripleServerBaseType, " struct {")
	g.P("proxyImpl protocol.Invoker")
	g.P("}")
	g.P()

	// add set method
	//func (g *GreeterProviderBase) SetProxyImpl(impl protocol.Invoker) {
	//	g.proxyImpl = impl
	//}
	g.P("func (s *", tripleServerBaseType, ") SetProxyImpl(impl protocol.Invoker) {")
	g.P(`s.proxyImpl = impl`)
	g.P("}")
	g.P()

	// return get method
	g.P("func (s *", tripleServerBaseType, ") GetProxyImpl() protocol.Invoker {")
	g.P(`return s.proxyImpl`)
	g.P("}")
	g.P()

	// return reference
	g.P("func (c *", tripleServerBaseType, ") ", " Reference() string ", "{")
	g.P(`return "`, unexport(servName), `Impl"`)
	g.P("}")
	g.P()

	// add handler
	var handlerNames []string
	for _, method := range service.Method {
		hname := g.generateServerMethod(servName, fullServName, method)
		handlerNames = append(handlerNames, hname)
	}

	grpcserverType := servName + "Server"
	// return service desc
	g.P("func (s *", tripleServerBaseType, ") ServiceDesc() *grpc.ServiceDesc {")
	g.P(`return &grpc.ServiceDesc{`)
	g.P("ServiceName: ", strconv.Quote(fullServName), ",")
	g.P("HandlerType: (*", grpcserverType, ")(nil),")
	g.P("Methods: []", grpcPkg, ".MethodDesc{")
	for i, method := range service.Method {
		if method.GetServerStreaming() || method.GetClientStreaming() {
			continue
		}
		g.P("{")
		g.P("MethodName: ", strconv.Quote(method.GetName()), ",")
		g.P("Handler: ", handlerNames[i], ",")
		g.P("},")
	}
	g.P("},")
	g.P("Streams: []", grpcPkg, ".StreamDesc{")
	for i, method := range service.Method {
		if !method.GetClientStreaming() && !method.GetServerStreaming() {
			continue
		}
		g.P("{")
		g.P("StreamName: ", strconv.Quote(method.GetName()), ",")
		g.P("Handler: ", handlerNames[i], ",")
		if method.GetServerStreaming() {
			g.P("ServerStreams: true,")
		}
		if method.GetClientStreaming() {
			g.P("ClientStreams: true,")
		}
		g.P("},")
	}
	g.P("},")
	g.P("Metadata: \"", file.GetName(), "\",")
	g.P("}")
	g.P("}")
	g.P()
}

// generateTripleClientSignature returns the client-side signature for a method.
func (g *triple) generateTripleClientSignature(servName string, method *pb.MethodDescriptorProto) string {
	origMethName := method.GetName()
	methName := generator.CamelCase(origMethName)
	if reservedClientName[methName] {
		methName += "_"
	}
	reqArg := ", in *" + g.typeName(method.GetInputType())
	if method.GetClientStreaming() {
		reqArg = ""
	}
	respName := "*" + g.typeName(method.GetOutputType())
	if method.GetServerStreaming() || method.GetClientStreaming() {
		respName = servName + "_" + generator.CamelCase(origMethName) + "Client"
		return fmt.Sprintf("%s(ctx %s.Context%s, opts ...%s.CallOption) (%s, error)", methName, contextPkg, reqArg, grpcPkg, respName)
	}
	return fmt.Sprintf("%s(ctx %s.Context%s, opts ...%s.CallOption) (%s, common.ErrorWithAttachment)", methName, contextPkg, reqArg, grpcPkg, respName)
}

// generateClientSignature returns the client-side signature for a method.
func (g *triple) generateClientSignature(servName string, method *pb.MethodDescriptorProto) string {
	origMethName := method.GetName()
	methName := generator.CamelCase(origMethName)
	//if reservedClientName[methName] {
	//  methName += "_"
	//}
	reqArg := ", in *" + g.typeName(method.GetInputType())
	if method.GetClientStreaming() {
		reqArg = ""
	}
	respName := "out *" + g.typeName(method.GetOutputType())
	if method.GetServerStreaming() || method.GetClientStreaming() {
		respName = servName + "_" + generator.CamelCase(origMethName) + "Client"
		return fmt.Sprintf("%s func(ctx %s.Context%s) (%s, error)", methName, contextPkg, reqArg, respName)
	}
	return fmt.Sprintf("%s func(ctx %s.Context%s) (*%s, error)", methName, contextPkg, reqArg, g.typeName(method.GetOutputType()))
}

// reservedClientName records whether a client name is reserved on the client side.
var reservedClientName = map[string]bool{
	// TODO: do we need any in gRPC?
}

//generateTripleServerSignature returns the server-side signature for a method.
func (g *triple) generateTripleServerSignature(servName string, method *pb.MethodDescriptorProto) string {
	origMethName := method.GetName()
	methName := generator.CamelCase(origMethName)
	if reservedClientName[methName] {
		methName += "_"
	}

	var reqArgs []string
	ret := "error"
	if !method.GetServerStreaming() && !method.GetClientStreaming() {
		reqArgs = append(reqArgs, contextPkg+".Context")
		ret = "(*" + g.typeName(method.GetOutputType()) + ", error)"
	}
	if !method.GetClientStreaming() {
		reqArgs = append(reqArgs, "*"+g.typeName(method.GetInputType()))
	}
	if method.GetServerStreaming() || method.GetClientStreaming() {
		reqArgs = append(reqArgs, servName+"_"+generator.CamelCase(origMethName)+"Server")
	}

	return methName + "(" + strings.Join(reqArgs, ", ") + ") " + ret
}

func (g *triple) generateClientMethod(servName, fullServName, serviceDescVar string, method *pb.MethodDescriptorProto, descExpr string) {
	methName := generator.CamelCase(method.GetName())
	streamType := unexport(servName) + methName + "Client"
	inType := g.typeName(method.GetInputType())
	outType := g.typeName(method.GetOutputType())

	genSend := method.GetClientStreaming()
	genRecv := method.GetServerStreaming()
	genCloseAndRecv := !method.GetServerStreaming()

	// Stream auxiliary types and methods.
	g.P("type ", servName, "_", methName, "Client interface {")
	if genSend {
		g.P("Send(*", inType, ") error")
	}
	if genRecv {
		g.P("Recv() (*", outType, ", error)")
	}
	if genCloseAndRecv {
		g.P("CloseAndRecv() (*", outType, ", error)")
	}
	g.P(grpcPkg, ".ClientStream")
	g.P("}")
	g.P()

	g.P("type ", streamType, " struct {")
	g.P(grpcPkg, ".ClientStream")
	g.P("}")
	g.P()

	if genSend {
		g.P("func (x *", streamType, ") Send(m *", inType, ") error {")
		g.P("return x.ClientStream.SendMsg(m)")
		g.P("}")
		g.P()
	}
	if genRecv {
		g.P("func (x *", streamType, ") Recv() (*", outType, ", error) {")
		g.P("m := new(", outType, ")")
		g.P("if err := x.ClientStream.RecvMsg(m); err != nil { return nil, err }")
		g.P("return m, nil")
		g.P("}")
		g.P()
	}
	if genCloseAndRecv {
		g.P("func (x *", streamType, ") CloseAndRecv() (*", outType, ", error) {")
		g.P("if err := x.ClientStream.CloseSend(); err != nil { return nil, err }")
		g.P("m := new(", outType, ")")
		g.P("if err := x.ClientStream.RecvMsg(m); err != nil { return nil, err }")
		g.P("return m, nil")
		g.P("}")
		g.P()
	}
}

func (g *triple) generateServerMethod(servName, fullServName string, method *pb.MethodDescriptorProto) string {
	methName := generator.CamelCase(method.GetName())
	hname := fmt.Sprintf("_Triple_%s_%s_Handler", servName, methName)
	inType := g.typeName(method.GetInputType())

	if !method.GetServerStreaming() && !method.GetClientStreaming() {
		g.P("func ", hname, "(srv interface{}, ctx ", contextPkg, ".Context, dec func(interface{}) error, interceptor ", grpcPkg, ".UnaryServerInterceptor) (interface{}, error) {")
		g.P("in := new(", inType, ")")
		g.P("if err := dec(in); err != nil { return nil, err }")

		g.P("base := srv.(dgrpc.Dubbo3GrpcService)")
		g.P("args := []interface{}{}")
		g.P("args = append(args, in)")
		g.P(`invo := invocation.NewRPCInvocation("`, methName, `", args, nil)`)

		g.P("if interceptor == nil {")
		g.P("result := base.GetProxyImpl().Invoke(ctx, invo)")
		g.P("return result, result.Error()")
		g.P("}")

		g.P("info := &", grpcPkg, ".UnaryServerInfo{")
		g.P("Server: srv,")
		g.P("FullMethod: ", strconv.Quote(fmt.Sprintf("/%s/%s", fullServName, methName)), ",")
		g.P("}")

		g.P("handler := func(ctx ", contextPkg, ".Context, req interface{}) (interface{}, error) {")
		g.P("result := base.GetProxyImpl().Invoke(context.Background(), invo)")
		g.P("return result.Result(), result.Error()")
		g.P("}")

		g.P("return interceptor(ctx, in, info, handler)")
		g.P("}")
		g.P()
		return hname
	}
	// streaming rpc
	streamType := unexport(servName) + methName + "Server"
	g.P("func ", hname, "(srv interface{}, stream ", grpcPkg, ".ServerStream) error {")
	g.P("_, ok := srv.(dgrpc.Dubbo3GrpcService)")
	g.P(`invo := invocation.NewRPCInvocation("`, methName, `", nil, nil)`)
	g.P("if !ok {")
	g.P("fmt.Println(invo)")
	g.P(`return nil`)
	g.P("}")
	if !method.GetClientStreaming() {
		g.P("m := new(", inType, ")")
		g.P("if err := stream.RecvMsg(m); err != nil { return err }")
		g.P("return srv.(", servName, "Server).", methName, "(m, &", streamType, "{stream})")
	} else {
		g.P("return srv.(", servName, "Server).", methName, "(&", streamType, "{stream})")
	}
	g.P("}")
	g.P()

	outType := g.typeName(method.GetOutputType())
	genSend := method.GetServerStreaming()
	genSendAndClose := !method.GetServerStreaming()
	genRecv := method.GetClientStreaming()

	// Stream auxiliary types and methods.
	g.P("type ", servName, "_", methName, "Server interface {")
	if genSend {
		g.P("Send(*", outType, ") error")
	}
	if genSendAndClose {
		g.P("SendAndClose(*", outType, ") error")
	}
	if genRecv {
		g.P("Recv() (*", inType, ", error)")
	}
	g.P(grpcPkg, ".ServerStream")
	g.P("}")
	g.P()

	g.P("type ", streamType, " struct {")
	g.P(grpcPkg, ".ServerStream")
	g.P("}")
	g.P()

	if genSend {
		g.P("func (x *", streamType, ") Send(m *", outType, ") error {")
		g.P("return x.ServerStream.SendMsg(m)")
		g.P("}")
		g.P()
	}
	if genSendAndClose {
		g.P("func (x *", streamType, ") SendAndClose(m *", outType, ") error {")
		g.P("return x.ServerStream.SendMsg(m)")
		g.P("}")
		g.P()
	}
	if genRecv {
		g.P("func (x *", streamType, ") Recv() (*", inType, ", error) {")
		g.P("m := new(", inType, ")")
		g.P("if err := x.ServerStream.RecvMsg(m); err != nil { return nil, err }")
		g.P("return m, nil")
		g.P("}")
		g.P()
	}

	return hname
}
