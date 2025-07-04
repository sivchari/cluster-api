/*
Copyright 2025 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tree

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	. "github.com/onsi/gomega"
	gtype "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client/tree"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"
)

func Test_getRowName(t *testing.T) {
	tests := []struct {
		name   string
		object ctrlclient.Object
		expect string
	}{
		{
			name:   "Row name for objects should be kind/name",
			object: fakeObject("c1"),
			expect: "Object/c1",
		},
		{
			name:   "Row name for a deleting object should have deleted prefix",
			object: fakeObject("c1", withDeletionTimestamp),
			expect: "!! DELETED !! Object/c1",
		},
		{
			name:   "Row name for objects with meta name should be meta-name - kind/name",
			object: fakeObject("c1", withAnnotation(tree.ObjectMetaNameAnnotation, "MetaName")),
			expect: "MetaName - Object/c1",
		},
		{
			name:   "Row name for virtual objects should be name",
			object: fakeObject("c1", withAnnotation(tree.VirtualObjectAnnotation, "True")),
			expect: "c1",
		},
		{
			name: "Row name for group objects should be #-of-items kind",
			object: fakeObject("c1",
				withAnnotation(tree.VirtualObjectAnnotation, "True"),
				withAnnotation(tree.GroupObjectAnnotation, "True"),
				withAnnotation(tree.GroupItemsAnnotation, "c1, c2, c3"),
			),
			expect: "3 Objects...",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			got := getRowName(tt.object)
			g.Expect(got).To(Equal(tt.expect))
		})
	}
}

func Test_newConditionDescriptor_readyColor(t *testing.T) {
	tests := []struct {
		name             string
		condition        *clusterv1.Condition
		expectReadyColor *color.Color
	}{
		{
			name:             "True condition should be green",
			condition:        v1beta1conditions.TrueCondition("C"),
			expectReadyColor: green,
		},
		{
			name:             "Unknown condition should be white",
			condition:        v1beta1conditions.UnknownCondition("C", "", ""),
			expectReadyColor: white,
		},
		{
			name:             "False condition, severity error should be red",
			condition:        v1beta1conditions.FalseCondition("C", "", clusterv1.ConditionSeverityError, ""),
			expectReadyColor: red,
		},
		{
			name:             "False condition, severity warning should be yellow",
			condition:        v1beta1conditions.FalseCondition("C", "", clusterv1.ConditionSeverityWarning, ""),
			expectReadyColor: yellow,
		},
		{
			name:             "False condition, severity info should be white",
			condition:        v1beta1conditions.FalseCondition("C", "", clusterv1.ConditionSeverityInfo, ""),
			expectReadyColor: white,
		},
		{
			name:             "Condition without status should be gray",
			condition:        &clusterv1.Condition{},
			expectReadyColor: gray,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			got := newV1Beta1ConditionDescriptor(tt.condition)
			g.Expect(got.readyColor).To(Equal(tt.expectReadyColor))
		})
	}
}

func Test_newConditionDescriptor_truncateMessages(t *testing.T) {
	tests := []struct {
		name          string
		condition     *clusterv1.Condition
		expectMessage string
	}{
		{
			name:          "Short messages are not changed",
			condition:     v1beta1conditions.UnknownCondition("C", "", "short message"),
			expectMessage: "short message",
		},
		{
			name:          "Long message are truncated",
			condition:     v1beta1conditions.UnknownCondition("C", "", "%s", strings.Repeat("s", 150)),
			expectMessage: fmt.Sprintf("%s ...", strings.Repeat("s", 100)),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			got := newV1Beta1ConditionDescriptor(tt.condition)
			g.Expect(got.message).To(Equal(tt.expectMessage))
		})
	}
}

func Test_V1Beta1TreePrefix(t *testing.T) {
	tests := []struct {
		name         string
		objectTree   *tree.ObjectTree
		expectPrefix []string
	}{
		{
			name: "First level child should get the right prefix",
			objectTree: func() *tree.ObjectTree {
				root := fakeObject("root")
				obectjTree := tree.NewObjectTree(root, tree.ObjectTreeOptions{})

				o1 := fakeObject("child1")
				o2 := fakeObject("child2")
				obectjTree.Add(root, o1)
				obectjTree.Add(root, o2)
				return obectjTree
			}(),
			expectPrefix: []string{
				"Object/root",
				"├─Object/child1", // first objects gets ├─
				"└─Object/child2", // last objects gets └─
			},
		},
		{
			name: "Second level child should get the right prefix",
			objectTree: func() *tree.ObjectTree {
				root := fakeObject("root")
				obectjTree := tree.NewObjectTree(root, tree.ObjectTreeOptions{})

				o1 := fakeObject("child1")
				o1_1 := fakeObject("child1.1")
				o1_2 := fakeObject("child1.2")
				o2 := fakeObject("child2")
				o2_1 := fakeObject("child2.1")
				o2_2 := fakeObject("child2.2")

				obectjTree.Add(root, o1)
				obectjTree.Add(o1, o1_1)
				obectjTree.Add(o1, o1_2)
				obectjTree.Add(root, o2)
				obectjTree.Add(o2, o2_1)
				obectjTree.Add(o2, o2_2)
				return obectjTree
			}(),
			expectPrefix: []string{
				"Object/root",
				"├─Object/child1",
				"│ ├─Object/child1.1", // first second level child gets pipes and ├─
				"│ └─Object/child1.2", // last second level child gets pipes and └─
				"└─Object/child2",
				"  ├─Object/child2.1", // first second level child spaces and ├─
				"  └─Object/child2.2", // last second level child gets spaces and └─
			},
		},
		{
			name: "Conditions should get the right prefix",
			objectTree: func() *tree.ObjectTree {
				root := fakeObject("root")
				obectjTree := tree.NewObjectTree(root, tree.ObjectTreeOptions{})

				o1 := fakeObject("child1",
					withAnnotation(tree.ShowObjectConditionsAnnotation, "True"),
					withV1Beta1Condition(v1beta1conditions.TrueCondition("C1.1")),
					withV1Beta1Condition(v1beta1conditions.TrueCondition("C1.2")),
				)
				o2 := fakeObject("child2",
					withAnnotation(tree.ShowObjectConditionsAnnotation, "True"),
					withV1Beta1Condition(v1beta1conditions.TrueCondition("C2.1")),
					withV1Beta1Condition(v1beta1conditions.TrueCondition("C2.2")),
				)
				obectjTree.Add(root, o1)
				obectjTree.Add(root, o2)
				return obectjTree
			}(),
			expectPrefix: []string{
				"Object/root",
				"├─Object/child1",
				"│             ├─C1.1", // first condition child gets pipes and ├─
				"│             └─C1.2", // last condition child gets └─ and pipes and └─
				"└─Object/child2",
				"              ├─C2.1", // first condition child gets spaces and ├─
				"              └─C2.2", // last condition child gets spaces and └─
			},
		},
		{
			name: "Conditions should get the right prefix if the object has a child",
			objectTree: func() *tree.ObjectTree {
				root := fakeObject("root")
				obectjTree := tree.NewObjectTree(root, tree.ObjectTreeOptions{})

				o1 := fakeObject("child1",
					withAnnotation(tree.ShowObjectConditionsAnnotation, "True"),
					withV1Beta1Condition(v1beta1conditions.TrueCondition("C1.1")),
					withV1Beta1Condition(v1beta1conditions.TrueCondition("C1.2")),
				)
				o1_1 := fakeObject("child1.1")

				o2 := fakeObject("child2",
					withAnnotation(tree.ShowObjectConditionsAnnotation, "True"),
					withV1Beta1Condition(v1beta1conditions.TrueCondition("C2.1")),
					withV1Beta1Condition(v1beta1conditions.TrueCondition("C2.2")),
				)
				o2_1 := fakeObject("child2.1")
				obectjTree.Add(root, o1)
				obectjTree.Add(o1, o1_1)
				obectjTree.Add(root, o2)
				obectjTree.Add(o2, o2_1)
				return obectjTree
			}(),
			expectPrefix: []string{
				"Object/root",
				"├─Object/child1",
				"│ │           ├─C1.1", // first condition child gets pipes, children pipe and ├─
				"│ │           └─C1.2", // last condition child gets pipes, children pipe and └─
				"│ └─Object/child1.1",
				"└─Object/child2",
				"  │           ├─C2.1", // first condition child gets spaces, children pipe and ├─
				"  │           └─C2.2", // last condition child gets spaces, children pipe and └─
				"  └─Object/child2.1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			var output bytes.Buffer

			// Creates the output table
			tbl := tablewriter.NewWriter(&output)

			formatTableTreeV1Beta1(tbl)

			// Add row for the root object, the cluster, and recursively for all the nodes representing the cluster status.
			addObjectRowV1Beta1("", tbl, tt.objectTree, tt.objectTree.GetRoot())
			tbl.Render()

			// Compare the output with the expected prefix.
			// We only check whether the output starts with the expected prefix,
			// meaning expectPrefix does not contain the full expected output.
			g.Expect(output.String()).Should(MatchTable(tt.expectPrefix))
		})
	}
}

func Test_TreePrefix(t *testing.T) {
	tests := []struct {
		name         string
		objectTree   *tree.ObjectTree
		expectPrefix []string
	}{
		{
			name: "Conditions should get the right prefix with multiline message",
			objectTree: func() *tree.ObjectTree {
				root := fakeObject("root",
					withCondition(metav1.Condition{
						Type:    "Available",
						Status:  metav1.ConditionFalse,
						Reason:  "NotAvailable",
						Message: "first line must not be validated in the test\nsecond line",
					}),
				)
				objectTree := tree.NewObjectTree(root, tree.ObjectTreeOptions{})

				o1 := fakeObject("child1",
					withCondition(falseCondition("Available", "first line must not be validated in the test\nsecond line")),
				)

				o2 := fakeObject("child2",
					withCondition(falseCondition("Available", "first line must not be validated in the test\nsecond line")),
				)
				objectTree.Add(root, o1)
				objectTree.Add(root, o2)
				return objectTree
			}(),
			expectPrefix: []string{
				"Object/root              Available: False  NotAvailable",
				"│                                                              second line",
				"├─Object/child1          Available: False  NotAvailable",
				"│                                                              second line",
				"└─Object/child2          Available: False  NotAvailable",
				"                                                               second line",
			},
		},
		{
			name: "Conditions should get the right prefix with multiline message and a child",
			objectTree: func() *tree.ObjectTree {
				root := fakeObject("root",
					withCondition(metav1.Condition{
						Type:   "Available",
						Status: metav1.ConditionTrue,
						Reason: "Available",
					}),
				)
				obectjTree := tree.NewObjectTree(root, tree.ObjectTreeOptions{})

				o1 := fakeObject("child1",
					withCondition(trueCondition()),
				)

				o2 := fakeObject("child2",
					withCondition(falseCondition("Available", "first line must not be validated in the test\nsecond line")),
				)
				o2_1 := fakeObject("child2.1")
				obectjTree.Add(root, o1)
				obectjTree.Add(root, o2)
				obectjTree.Add(o2, o2_1)
				return obectjTree
			}(),
			expectPrefix: []string{
				"Object/root                  Available: True   Available",
				"├─Object/child1              Available: True   Available",
				"└─Object/child2              Available: False  NotAvailable",
				"  │                                                                second line",
				"  └─Object/child2.1",
			},
		},
		{
			name: "Multiple nested childs should get the right multiline prefix",
			objectTree: func() *tree.ObjectTree {
				root := fakeObject("root",
					withCondition(metav1.Condition{
						Type:   "Available",
						Status: metav1.ConditionTrue,
						Reason: "Available",
					}),
				)
				obectjTree := tree.NewObjectTree(root, tree.ObjectTreeOptions{})

				o1 := fakeObject("child1",
					withCondition(trueCondition()),
				)
				o2 := fakeObject("child2",
					withCondition(falseCondition("Available", "first line must not be validated in the test\nsecond line")),
				)
				o3 := fakeObject("child3",
					withCondition(falseCondition("Available", "first line must not be validated in the test\nsecond line")),
				)
				o4 := fakeObject("child4",
					withCondition(falseCondition("Available", "first line must not be validated in the test\nsecond line")),
				)
				o5 := fakeObject("child5",
					withCondition(falseCondition("Available", "first line must not be validated in the test\nsecond line")),
				)
				obectjTree.Add(root, o1)
				obectjTree.Add(o1, o2)
				obectjTree.Add(o2, o3)
				obectjTree.Add(o3, o4)
				obectjTree.Add(root, o5)
				return obectjTree
			}(),
			expectPrefix: []string{
				"Object/root                    Available: True   Available",
				"├─Object/child1                Available: True   Available",
				"│ └─Object/child2              Available: False  NotAvailable",
				"│   │                                                                second line",
				"│   └─Object/child3            Available: False  NotAvailable",
				"│     │                                                              second line",
				"│     └─Object/child4          Available: False  NotAvailable",
				"│                                                                    second line",
				"└─Object/child5                Available: False  NotAvailable",
				"                                                                     second line",
			},
		},
		{
			name: "Nested childs should get the right prefix with multiline message",
			objectTree: func() *tree.ObjectTree {
				root := fakeObject("root",
					withCondition(metav1.Condition{
						Type:   "Available",
						Status: metav1.ConditionTrue,
						Reason: "Available",
					}),
				)
				obectjTree := tree.NewObjectTree(root, tree.ObjectTreeOptions{})

				o1 := fakeObject("child1",
					withCondition(trueCondition()),
				)
				o2 := fakeObject("child2",
					withCondition(falseCondition("Available", "first line must not be validated in the test\nsecond line")),
				)
				o3 := fakeObject("child3",
					withCondition(falseCondition("Available", "first line must not be validated in the test\nsecond line")),
				)
				o4 := fakeObject("child4",
					withCondition(falseCondition("Available", "first line must not be validated in the test\nsecond line")),
				)
				obectjTree.Add(root, o1)
				obectjTree.Add(o1, o2)
				obectjTree.Add(o2, o3)
				obectjTree.Add(o3, o4)
				return obectjTree
			}(),
			expectPrefix: []string{
				"Object/root                    Available: True   Available",
				"└─Object/child1                Available: True   Available",
				"  └─Object/child2              Available: False  NotAvailable",
				"    │                                                                second line",
				"    └─Object/child3            Available: False  NotAvailable",
				"      │                                                              second line",
				"      └─Object/child4          Available: False  NotAvailable",
				"                                                                     second line",
			},
		},
		{
			name: "Conditions should get the right prefix with childs",
			objectTree: func() *tree.ObjectTree {
				root := fakeObject("root",
					withCondition(metav1.Condition{
						Type:   "Available",
						Status: metav1.ConditionTrue,
						Reason: "Available",
					}),
				)
				obectjTree := tree.NewObjectTree(root, tree.ObjectTreeOptions{})

				o1 := fakeObject("child1",
					withCondition(trueCondition()),
				)

				o2 := fakeObject("child2",
					withCondition(falseCondition("Available", "first line must not be validated in the test\nsecond line")),
				)
				o2_1 := fakeObject("child2.1",
					withCondition(falseCondition("Available", "first line must not be validated in the test\nsecond line")),
				)
				o3 := fakeObject("child3",
					withCondition(falseCondition("Available", "first line must not be validated in the test\nsecond line")),
				)
				o3_1 := fakeObject("child3.1",
					withCondition(falseCondition("Available", "first line must not be validated in the test\nsecond line")),
				)
				obectjTree.Add(root, o1)
				obectjTree.Add(root, o2)
				obectjTree.Add(o2, o2_1)
				obectjTree.Add(root, o3)
				obectjTree.Add(o3, o3_1)
				return obectjTree
			}(),
			expectPrefix: []string{
				"Object/root                  Available: True   Available",
				"├─Object/child1              Available: True   Available",
				"├─Object/child2              Available: False  NotAvailable",
				"│ │                                                                second line",
				"│ └─Object/child2.1          Available: False  NotAvailable",
				"│                                                                  second line",
				"└─Object/child3              Available: False  NotAvailable",
				"  │                                                                second line",
				"  └─Object/child3.1          Available: False  NotAvailable",
				"                                                                   second line",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			var output bytes.Buffer

			// Creates the output table
			tbl := tablewriter.NewWriter(&output)

			formatTableTree(tbl)

			// Add row for the root object, the cluster, and recursively for all the nodes representing the cluster status.
			addObjectRow("", tbl, tt.objectTree, tt.objectTree.GetRoot())
			tbl.Render()

			// Remove empty lines from the output. We need this because v1beta2 adds lines at the beginning and end.
			outputString := strings.TrimSpace(output.String())

			// Compare the output with the expected prefix.
			// We only check whether the output starts with the expected prefix,
			// meaning expectPrefix does not contain the full expected output.
			g.Expect(outputString).Should(MatchTable(tt.expectPrefix))
		})
	}
}

type objectOption func(object ctrlclient.Object)

func fakeObject(name string, options ...objectOption) ctrlclient.Object {
	c := &clusterv1.Cluster{ // suing type cluster for simplicity, but this could be any object
		TypeMeta: metav1.TypeMeta{
			Kind: "Object",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns",
			Name:      name,
			UID:       types.UID(name),
		},
	}
	for _, opt := range options {
		opt(c)
	}
	return c
}

func withAnnotation(name, value string) func(ctrlclient.Object) {
	return func(c ctrlclient.Object) {
		if c.GetAnnotations() == nil {
			c.SetAnnotations(map[string]string{})
		}
		a := c.GetAnnotations()
		a[name] = value
		c.SetAnnotations(a)
	}
}

func withV1Beta1Condition(c *clusterv1.Condition) func(ctrlclient.Object) {
	return func(m ctrlclient.Object) {
		setter := m.(v1beta1conditions.Setter)
		v1beta1conditions.Set(setter, c)
	}
}

func withCondition(c metav1.Condition) func(ctrlclient.Object) {
	return func(m ctrlclient.Object) {
		cluster := m.(*clusterv1.Cluster)
		conds := cluster.GetConditions()
		conds = append(conds, c)
		cluster.SetConditions(conds)
	}
}

func trueCondition() metav1.Condition {
	return metav1.Condition{
		Type:   "Available",
		Status: metav1.ConditionTrue,
		Reason: "Available",
	}
}

func falseCondition(t, m string) metav1.Condition {
	return metav1.Condition{
		Type:    t,
		Status:  metav1.ConditionFalse,
		Reason:  "Not" + t,
		Message: m,
	}
}

func withDeletionTimestamp(object ctrlclient.Object) {
	now := metav1.Now()
	object.SetDeletionTimestamp(&now)
}

type Table struct {
	tableData []string
}

func MatchTable(expected []string) gtype.GomegaMatcher {
	return &Table{tableData: expected}
}

func (t *Table) Match(actual interface{}) (bool, error) {
	tableString := actual.(string)
	tableParts := strings.Split(tableString, "\n")

	for i := range t.tableData {
		if !strings.HasPrefix(tableParts[i], t.tableData[i]) {
			return false, nil
		}
	}
	return true, nil
}

func (t *Table) FailureMessage(actual interface{}) string {
	actualTable := strings.Split(actual.(string), "\n")
	return fmt.Sprintf("Expected %v and received %v", t.tableData, actualTable)
}

func (t *Table) NegatedFailureMessage(actual interface{}) string {
	actualTable := strings.Split(actual.(string), "\n")
	return fmt.Sprintf("Expected %v and received %v", t.tableData, actualTable)
}

func Test_formatParagraph(t *testing.T) {
	tests := []struct {
		text     string
		maxWidth int
		want     string
	}{
		{
			text:     "",
			maxWidth: 254,
			want:     "",
		},
		{
			text:     "* a b c d e f",
			maxWidth: 5,
			want:     "* a b\n  c d\n  e f",
		},
		{
			text: "* a b c d e f\n" +
				"  * g h\n" +
				"  * i j",
			maxWidth: 5,
			want: "* a b\n" +
				"  c d\n" +
				"  e f\n" +
				"  * g\n" +
				"    h\n" +
				"  * i\n" +
				"    j",
		},
	}
	for ti, tt := range tests {
		t.Run(fmt.Sprintf("%d", ti), func(t *testing.T) {
			g := NewWithT(t)
			g.Expect("\n" + formatParagraph(tt.text, tt.maxWidth)).To(BeEquivalentTo("\n" + tt.want))
		})
	}
}
