package tool_git_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"arhat.dev/dukkha/pkg/dukkha"
	dukkha_test "arhat.dev/dukkha/pkg/dukkha/test"
	tool_git "arhat.dev/dukkha/pkg/tools/git"
	"arhat.dev/dukkha/pkg/tools/tests"
)

func TestTaskClone_GetExecSpecs(t *testing.T) {
	toolCmd := []string{"git"}
	testCases := []tests.ExecSpecGenerationTestCase{
		{
			Name:      "Invalid Empty",
			Task:      &tool_git.TaskClone{},
			ExpectErr: true,
			Options:   dukkha_test.CreateTaskMatrixExecOptions(toolCmd),
		},
		{
			Name:    "Valid Clone Using Default Branch",
			Task:    &tool_git.TaskClone{URL: "example/foo.git"},
			Options: dukkha_test.CreateTaskMatrixExecOptions(toolCmd),
			Expected: []dukkha.TaskExecSpec{
				{
					Command: strings.Split("git clone --no-checkout --origin origin example/foo.git", " "),
				},
				{
					StdoutAsReplace: "<DEFAULT_BRANCH>",
					Chdir:           "foo",
					Command:         strings.Split("git symbolic-ref refs/remotes/origin/HEAD", " "),
				},
				{
					Chdir:   "foo",
					Command: strings.Split("git checkout -b <DEFAULT_BRANCH> origin/<DEFAULT_BRANCH>", " "),
				},
			},
		},
		{
			Name:    "Valid Clone Changing Remote Name",
			Task:    &tool_git.TaskClone{URL: "example/foo", RemoteName: "bar"},
			Options: dukkha_test.CreateTaskMatrixExecOptions(toolCmd),
			Expected: []dukkha.TaskExecSpec{
				{
					Command: strings.Split("git clone --no-checkout --origin bar example/foo", " "),
				},
				{
					StdoutAsReplace: "<DEFAULT_BRANCH>",
					Chdir:           "foo",
					Command:         strings.Split("git symbolic-ref refs/remotes/bar/HEAD", " "),
				},
				{
					Chdir:   "foo",
					Command: strings.Split("git checkout -b <DEFAULT_BRANCH> bar/<DEFAULT_BRANCH>", " "),
				},
			},
		},
	}

	tests.RunTaskExecSpecGenerationTests(
		t,
		dukkha_test.NewTestContext(t, context.TODO()),
		testCases,
	)

	assert.EqualValues(t, []string{"git"}, toolCmd)
}
