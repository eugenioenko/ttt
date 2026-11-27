package ui

import (
	"fmt"
	"testing"

	"github.com/eugenioenko/ttt/internal/widgets"
)

func BenchmarkExplorerSearchLoadedNodes(b *testing.B) {
	for _, count := range []int{1000, 10000, 100000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			roots := make([]*widgets.TreeNode, 0, count/100)
			for i := 0; i < count; i += 100 {
				root := &widgets.TreeNode{ID: fmt.Sprint(i), Label: fmt.Sprintf("folder-%d", i), Expandable: true}
				for j := i; j < min(i+100, count); j++ {
					root.Children = append(root.Children, &widgets.TreeNode{ID: fmt.Sprint(j), Label: fmt.Sprintf("component-%06d_test.go", j)})
				}
				roots = append(roots, root)
			}
			s := NewExplorerSearchWidget(widgets.NewTreeWidget(widgets.TreeConfig{Items: roots}), "")
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				query := "component"
				if i%2 == 0 {
					query = "missing"
				}
				s.Input.SetText(query)
			}
		})
	}
}
