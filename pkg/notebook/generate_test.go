package notebook

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/tkestack/elastic-jupyter-operator/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	JupyterNotebookName      = "jupyternotebook-sample"
	JupyterNotebookNamespace = "default"
	DefaultContainerName     = "notebook"
	DefaultImage             = "busysandbox"
	DefaultImageWithGateway  = "jupyter/base-notebook:python-3.9.7"
	GatewayName              = "gateway"
	GatewayNamespace         = "default"
)

var (
	podSpec = v1.PodSpec{
		Containers: []v1.Container{
			{
				Name:  DefaultContainerName,
				Image: DefaultImage,
			},
		},
	}

	emptyNotebook = &v1alpha1.JupyterNotebook{
		ObjectMeta: metav1.ObjectMeta{
			Name:      JupyterNotebookName,
			Namespace: JupyterNotebookNamespace,
		},
	}

	notebookWithTemplate = &v1alpha1.JupyterNotebook{
		ObjectMeta: metav1.ObjectMeta{
			Name:      JupyterNotebookName,
			Namespace: JupyterNotebookNamespace,
		},
		Spec: v1alpha1.JupyterNotebookSpec{
			Template: &v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":          "notebook",
						"custom-label": "yes",
					},
					Annotations: map[string]string{"custom-annotation": "yes"},
				},
				Spec: podSpec,
			},
		},
	}

	notebookWithGateway = &v1alpha1.JupyterNotebook{
		ObjectMeta: metav1.ObjectMeta{
			Name:      JupyterNotebookName,
			Namespace: JupyterNotebookNamespace,
		},
		Spec: v1alpha1.JupyterNotebookSpec{
			Gateway: &v1.ObjectReference{
				Kind:      "JupyterGateway",
				Namespace: GatewayNamespace,
				Name:      GatewayName,
			},
		},
	}

	testPwd                  = "test"
	notebookWithAuthPassword = &v1alpha1.JupyterNotebook{
		ObjectMeta: metav1.ObjectMeta{
			Name:      JupyterNotebookName,
			Namespace: JupyterNotebookNamespace,
		},
		Spec: v1alpha1.JupyterNotebookSpec{
			Auth: &v1alpha1.JupyterAuth{
				Password: &testPwd,
			},
			Template: &v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "notebook"},
				},
				Spec: podSpec,
			},
		},
	}

	completeNotebook = &v1alpha1.JupyterNotebook{
		ObjectMeta: metav1.ObjectMeta{
			Name:      JupyterNotebookName,
			Namespace: JupyterNotebookNamespace,
		},
		Spec: v1alpha1.JupyterNotebookSpec{
			Gateway: &v1.ObjectReference{
				Kind:      "JupyterGateway",
				Namespace: GatewayNamespace,
				Name:      GatewayName,
			},
			Template: &v1.PodTemplateSpec{
				Spec: podSpec,
			},
		},
	}
)

func TestGenerate(t *testing.T) {
	type test struct {
		input       *v1alpha1.JupyterNotebook
		expectedErr error
		expectedGen *generator
	}

	tests := []test{
		{input: nil, expectedErr: errors.New("the notebook is null"), expectedGen: nil},
		{input: notebookWithTemplate, expectedErr: nil, expectedGen: &generator{nb: notebookWithTemplate}},
		{input: notebookWithGateway, expectedErr: nil, expectedGen: &generator{nb: notebookWithGateway}},
		{input: completeNotebook, expectedErr: nil, expectedGen: &generator{nb: completeNotebook}},
		{input: emptyNotebook, expectedErr: nil, expectedGen: &generator{nb: emptyNotebook}},
	}

	for _, tc := range tests {
		gen, err := newGenerator(tc.input)
		if (tc.expectedErr == nil && err != nil) || (tc.expectedErr != nil && !errors.Is(err, tc.expectedErr)) {
			t.Errorf("expected: %v, got: %v", tc.expectedErr, err)
		}
		if err == nil {
			if tc.expectedGen == nil && gen != nil {
				t.Errorf("expected nil generator, got: %v", gen)
			} else if tc.expectedGen != nil && gen != nil {
				if !reflect.DeepEqual(tc.expectedGen.nb, gen.nb) {
					t.Errorf("expected notebook: %v, got: %v", tc.expectedGen.nb, gen.nb)
				}
			}
		}
	}
}

func TestDesiredDeploymentWithoutOwner(t *testing.T) {
	type test struct {
		gen                 *generator
		expectedErr         error
		expectedImage       string
		expectedLabels      map[string]string
		expectedAnnotations map[string]string
		expectedArgs        []string
	}

	tests := []test{
		{
			gen:           &generator{nb: notebookWithTemplate},
			expectedErr:   nil,
			expectedImage: DefaultImage,
			expectedLabels: map[string]string{
				"app":          "notebook",
				"custom-label": "yes",
			},
			expectedAnnotations: map[string]string{
				"custom-annotation": "yes",
			},
			expectedArgs: nil,
		},
		{
			gen:           &generator{nb: notebookWithGateway},
			expectedErr:   nil,
			expectedImage: DefaultImageWithGateway,
			expectedArgs: []string{
				"start-notebook.sh",
				argumentGatewayURL,
				fmt.Sprintf("http://%s.%s:%d", GatewayName, GatewayNamespace, 8888),
			},
		},
		{
			gen:         &generator{nb: completeNotebook},
			expectedErr: nil, expectedImage: DefaultImage,
			expectedArgs: []string{
				argumentGatewayURL,
				fmt.Sprintf("http://%s.%s:%d", GatewayName, GatewayNamespace, 8888),
			},
		},
		{
			gen:         &generator{nb: emptyNotebook},
			expectedErr: errors.New("no gateway and template applied"),
		},
		{
			gen:         &generator{nb: notebookWithAuthPassword},
			expectedErr: nil, expectedImage: DefaultImage,
			expectedArgs: []string{argumentNotebookPassword, *notebookWithAuthPassword.Spec.Auth.Password},
		},
	}

	for i, tc := range tests {
		d, err := tc.gen.DesiredDeploymentWithoutOwner()
		if (tc.expectedErr == nil && err != nil) || (tc.expectedErr != nil && !errors.Is(err, tc.expectedErr)) {
			t.Errorf("expected: %v, got: %v", tc.expectedErr, err)
		}
		if err == nil {
			if tc.expectedImage != d.Spec.Template.Spec.Containers[0].Image {
				t.Errorf("expected: %v, got: %v", tc.expectedImage, d.Spec.Template.Spec.Containers[0].Image)
			}
			if len(tc.expectedArgs) != len(d.Spec.Template.Spec.Containers[0].Args) {
				t.Errorf("i= %d expected args length: %d, got: %d", i, len(tc.expectedArgs), len(d.Spec.Template.Spec.Containers[0].Args))
			} else {
				for j, arg := range tc.expectedArgs {
					if arg != d.Spec.Template.Spec.Containers[0].Args[j] {
						t.Errorf("i= %d, j= %d expected arg: %v, got: %v", i, j, arg, d.Spec.Template.Spec.Containers[0].Args[j])
					}
				}
			}
		}
		for k, v := range tc.expectedLabels {
			if v != d.Spec.Template.Labels[k] {
				t.Errorf("expected: %v, got: %v", v, d.Labels[k])
			}
		}
		for k, v := range tc.expectedAnnotations {
			if v != d.Spec.Template.Annotations[k] {
				t.Errorf("expected: %v, got: %v", v, d.Annotations[k])
			}
		}
	}
}

func TestLable(t *testing.T) {
	type test struct {
		gen      *generator
		expected string
	}

	tests := []test{
		{gen: &generator{nb: notebookWithTemplate}, expected: JupyterNotebookName},
	}

	for _, tc := range tests {
		mp := tc.gen.labels()
		if !reflect.DeepEqual(tc.expected, mp["notebook"]) {
			t.Errorf("expected: %v, got: %v", tc.expected, mp["notebook"])
		}
	}
}
