package integration

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestDockerPrerequisiteCheckerEvaluate(t *testing.T) {
	requiredImages := []string{"image-one", "image-two"}

	createSignature := func(name string, args []string) string {
		return name + " " + strings.Join(args, " ")
	}

	type testCase struct {
		name              string
		environment       map[string]string
		lookupError       error
		commandResponses  map[string]error
		contextCancelled  bool
		expectedSkip      string
		expectedError     bool
		expectedCallOrder []string
	}

	testCases := []testCase{
		{
			name: "force overrides all checks",
			environment: map[string]string{
				dockerForceEnvironmentVariable: "1",
			},
			expectedSkip:      "",
			expectedError:     false,
			expectedCallOrder: nil,
		},
		{
			name: "skip via environment variable",
			environment: map[string]string{
				dockerSkipEnvironmentVariableName: "true",
			},
			expectedSkip:      fmt.Sprintf("Docker integration tests disabled via %s.", dockerSkipEnvironmentVariableName),
			expectedError:     false,
			expectedCallOrder: nil,
		},
		{
			name:        "missing docker executable",
			lookupError: errors.New("cannot find executable"),
			expectedSkip: fmt.Sprintf(
				"Docker integration tests require the %s executable: %v",
				dockerExecutableName,
				errors.New("cannot find executable"),
			),
			expectedError:     false,
			expectedCallOrder: nil,
		},
		{
			name: "docker version failure",
			commandResponses: map[string]error{
				createSignature(dockerExecutableName, dockerVersionCommandArguments): errors.New("daemon offline"),
			},
			expectedSkip: fmt.Sprintf(
				"Docker daemon is unavailable: %v",
				errors.New("daemon offline"),
			),
			expectedError:     false,
			expectedCallOrder: []string{createSignature(dockerExecutableName, dockerVersionCommandArguments)},
		},
		{
			name: "missing builder image",
			commandResponses: map[string]error{
				createSignature(dockerExecutableName, dockerVersionCommandArguments):                   nil,
				createSignature(dockerExecutableName, []string{"image", "inspect", requiredImages[0]}): errors.New("image not found"),
			},
			expectedSkip: fmt.Sprintf(
				"Docker image %s is not available locally: %v",
				requiredImages[0],
				errors.New("image not found"),
			),
			expectedError: false,
			expectedCallOrder: []string{
				createSignature(dockerExecutableName, dockerVersionCommandArguments),
				createSignature(dockerExecutableName, []string{"image", "inspect", requiredImages[0]}),
			},
		},
		{
			name: "all checks succeed",
			commandResponses: map[string]error{
				createSignature(dockerExecutableName, dockerVersionCommandArguments):                   nil,
				createSignature(dockerExecutableName, []string{"image", "inspect", requiredImages[0]}): nil,
				createSignature(dockerExecutableName, []string{"image", "inspect", requiredImages[1]}): nil,
			},
			expectedSkip:  "",
			expectedError: false,
			expectedCallOrder: []string{
				createSignature(dockerExecutableName, dockerVersionCommandArguments),
				createSignature(dockerExecutableName, []string{"image", "inspect", requiredImages[0]}),
				createSignature(dockerExecutableName, []string{"image", "inspect", requiredImages[1]}),
			},
		},
		{
			name:              "context canceled before checks",
			contextCancelled:  true,
			expectedSkip:      "",
			expectedError:     true,
			expectedCallOrder: nil,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var recordedCalls []string
			lookupFunction := func(name string) (string, error) {
				if name != dockerExecutableName {
					return "", fmt.Errorf("unexpected executable lookup: %s", name)
				}
				if testCase.lookupError != nil {
					return "", testCase.lookupError
				}
				return "/usr/bin/docker", nil
			}

			commandRunner := func(ctx context.Context, name string, args ...string) error {
				signature := createSignature(name, args)
				recordedCalls = append(recordedCalls, signature)
				if testCase.commandResponses != nil {
					if response, exists := testCase.commandResponses[signature]; exists {
						return response
					}
				}
				return nil
			}

			environmentReader := func(key string) string {
				if testCase.environment == nil {
					return ""
				}
				return testCase.environment[key]
			}

			checker := dockerPrerequisiteChecker{
				lookupExecutable: lookupFunction,
				runCommand:       commandRunner,
				readEnvironment:  environmentReader,
			}

			var evaluationContext context.Context
			var cancel context.CancelFunc
			if testCase.contextCancelled {
				evaluationContext, cancel = context.WithCancel(context.Background())
				cancel()
			} else {
				evaluationContext = context.Background()
			}

			skipReason, err := checker.evaluate(evaluationContext, requiredImages)

			if testCase.expectedError && err == nil {
				t.Fatalf("expected error but received none")
			}
			if !testCase.expectedError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if skipReason != testCase.expectedSkip {
				t.Fatalf("unexpected skip reason: %q", skipReason)
			}
			if len(testCase.expectedCallOrder) > 0 {
				if !reflect.DeepEqual(testCase.expectedCallOrder, recordedCalls) {
					t.Fatalf("unexpected command sequence: got %v, want %v", recordedCalls, testCase.expectedCallOrder)
				}
			} else if len(recordedCalls) != 0 && !testCase.expectedError {
				t.Fatalf("expected no commands but recorded: %v", recordedCalls)
			}
		})
	}
}
