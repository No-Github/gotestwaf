package waf

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	gtw_grpc "github.com/wallarm/gotestwaf/internal/payload/placeholder/grpc"
	"github.com/wallarm/gotestwaf/tests/integration/config"
)

type grpcServer struct {
	errChan  chan<- error
	casesMap *config.TestCasesMap

	gtw_grpc.UnimplementedServiceFooBarServer
}

func (s *grpcServer) Foo(ctx context.Context, in *gtw_grpc.Request) (*gtw_grpc.Response, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		s.errChan <- errors.New("metadata not found")
	}

	headerValue := md.Get("X-GoTestWAF-Test")
	if len(headerValue) < 1 {
		s.errChan <- errors.New("couldn't get X-GoTestWAF-Test header value")
	}

	headerValues := strings.Split(headerValue[0], ",")

	var err error
	var set string
	var name string
	var placeholder string
	var placeholderValue string
	var encoder string
	var value string

	testCaseParameters := make(map[string]string)

	for _, value = range headerValues {
		kv := strings.Split(value, "=")

		if len(kv) < 2 {
			s.errChan <- errors.New("couldn't parse header value")
		} else {
			testCaseParameters[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}

	if set, ok = testCaseParameters["set"]; !ok {
		s.errChan <- errors.New("couldn't get `set` parameter of test case")
	}

	if name, ok = testCaseParameters["name"]; !ok {
		s.errChan <- errors.New("couldn't get `name` parameter of test case")
	}

	if placeholder, ok = testCaseParameters["placeholder"]; !ok {
		s.errChan <- errors.New("couldn't get `placeholder` parameter of test case")
	}

	if encoder, ok = testCaseParameters["encoder"]; !ok {
		s.errChan <- errors.New("couldn't get `encoder` parameter of test case")
	}

	placeholderValue = in.GetValue()

	switch encoder {
	case "Base64":
		value, err = decodeBase64(placeholderValue)
	case "Base64Flat":
		value, err = decodeBase64(placeholderValue)
	case "JSUnicode":
		value, err = decodeJSUnicode(placeholderValue)
	case "URL":
		value, err = decodeURL(placeholderValue)
	case "Plain":
		value, err = decodePlain(placeholderValue)
	case "XMLEntity":
		value, err = decodeXMLEntity(placeholderValue)
	default:
		s.errChan <- fmt.Errorf("unknown encoder: %s", encoder)
	}

	if err != nil {
		s.errChan <- fmt.Errorf("couldn't decode payload: %v", err)
	}

	testCase := fmt.Sprintf("%s-%s-%s-%s-%s", set, name, value, placeholder, encoder)
	if !s.casesMap.CheckTestCaseAvailability(testCase) {
		s.errChan <- fmt.Errorf("received unknown payload: %s", testCase)
	}

	if matched, _ := regexp.MatchString("bypassed", value); matched {
		return &gtw_grpc.Response{Value: "OK"}, nil
	} else if matched, _ = regexp.MatchString("blocked", value); matched {
		return nil, status.New(codes.PermissionDenied, "").Err()
	}

	return nil, status.New(codes.NotFound, "").Err()
}
