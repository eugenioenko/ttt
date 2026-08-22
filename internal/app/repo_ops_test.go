package app

import "testing"

func TestRepositoryResourcesForRepoTask(t *testing.T) {
	tests := []struct {
		name string
		ops  []RepoOp
		want RepositoryResource
	}{
		{name: "push", ops: []RepoOp{OpPush}, want: RepositoryWorktree},
		{name: "commit", ops: []RepoOp{OpCommit("message")}, want: RepositoryWorktree | RepositoryHistory},
		{name: "pull", ops: []RepoOp{OpPull}, want: RepositoryWorktree | RepositoryHistory},
		{name: "sync", ops: []RepoOp{OpPull, OpPush}, want: RepositoryWorktree | RepositoryHistory},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := repositoryResourcesForTask(RepoTask{Ops: test.ops}); got != test.want {
				t.Fatalf("resources = %b, want %b", got, test.want)
			}
		})
	}
}
