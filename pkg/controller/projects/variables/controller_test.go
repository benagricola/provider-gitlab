/*
Copyright 2021 The Crossplane Authors.

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

package variables

import (
	"context"
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/pkg/errors"

	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/xanzy/go-gitlab"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane-contrib/provider-gitlab/apis/projects/v1alpha1"
	"github.com/crossplane-contrib/provider-gitlab/pkg/clients/projects"
	"github.com/crossplane-contrib/provider-gitlab/pkg/clients/projects/fake"
)

var (
	errBoom       = errors.New("boom")
	projectID     = 5678
	variableKey   = "VARIABLE_KEY"
	variableValue = "1234"
)

type args struct {
	variable projects.VariableClient
	kube        client.Client
	cr          *v1alpha1.Variable
}

type variableModifier func(*v1alpha1.Variable)

func withConditions(c ...xpv1.Condition) variableModifier {
	return func(r *v1alpha1.Variable) { r.Status.ConditionedStatus.Conditions = c }
}

func withDefaultValues() variableModifier {
	return func(pv *v1alpha1.Variable) {
		pv.Spec.ForProvider = v1alpha1.VariableParameters{
			ProjectID: &projectID,
			Key: variableKey,
			Value: variableValue,
		}
	}
}

func withProjectID(pid int) variableModifier {
	return func(r *v1alpha1.Variable) {
		r.Spec.ForProvider.ProjectID = &pid
	}
}

func withValue(value string) variableModifier {
	return func(r *v1alpha1.Variable) {
		r.Spec.ForProvider.Value = value
	}
}

func withKey(key string) variableModifier {
	return func(r *v1alpha1.Variable) {
		r.Spec.ForProvider.Key = key
	}
}

func variable(m ...variableModifier) *v1alpha1.Variable {
	cr := &v1alpha1.Variable{}
	for _, f := range m {
		f(cr)
	}
	return cr
}

func TestObserve(t *testing.T) {
	type want struct {
		cr     *v1alpha1.Variable
		result managed.ExternalObservation
		err    error
	}

	cases := map[string]struct {
		args
		want
	}{
		"SuccessfulAvailable": {
			args: args{
				variable: &fake.MockClient{
					MockGetProjectVariable: func(pid interface{}, key string, options ...gitlab.RequestOptionFunc) (*gitlab.ProjectVariable, *gitlab.Response, error) {
						return &gitlab.ProjectVariable{}, &gitlab.Response{}, nil
					},
				},
				cr: variable(withDefaultValues()),
			},
			want: want{
				cr: variable(
					withDefaultValues(),
					withConditions(xpv1.Available()),
				),
				result: managed.ExternalObservation{
					ResourceExists:   true,
					ResourceUpToDate: true,
				},
			},
		},
		"NotUpToDate": {
			args: args{
				variable: &fake.MockClient{
					MockGetProjectVariable: func(pid interface{}, key string, options ...gitlab.RequestOptionFunc) (*gitlab.ProjectVariable, *gitlab.Response, error) {
						return &gitlab.ProjectVariable{
							Value: "test",
						}, &gitlab.Response{}, nil
					},
				},
				cr: variable(
					withDefaultValues(),
					withValue("blah"),
				),
			},
			want: want{
				cr: variable(
					withDefaultValues(),
					withValue("blah"),
				),
				result: managed.ExternalObservation{
					ResourceExists:   true,
					ResourceUpToDate: false,
				},
			},
		},
		"LateInitSuccess": {
			args: args{
				variable: &fake.MockClient{
					MockGetProjectVariable: func(pid interface{}, key string, options ...gitlab.RequestOptionFunc) (*gitlab.ProjectVariable, *gitlab.Response, error) {
						return &gitlab.ProjectVariable{}, &gitlab.Response{}, nil
					},
				},
				cr: variable(
					withProjectID(projectID),
					withKey(variableKey),
				),
			},
			want: want{
				cr: variable(
					withDefaultValues(),
					withKey(variableKey),
					withConditions(xpv1.Available()),
				),
				result: managed.ExternalObservation{
					ResourceExists:          true,
					ResourceUpToDate:        true,
					ResourceLateInitialized: true,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{kube: tc.kube, client: tc.variable}
			o, err := e.Observe(context.Background(), tc.args.cr)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.cr, tc.args.cr, test.EquateConditions()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.result, o); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestCreate(t *testing.T) {
	type want struct {
		cr     *v1alpha1.Variable
		result managed.ExternalCreation
		err    error
	}

	cases := map[string]struct {
		args
		want
	}{
		"SuccessfulCreation": {
			args: args{
				kube: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				variable: &fake.MockClient{
					MockCreateProjectVariable: func(pid interface{}, opt *gitlab.CreateProjectVariableOptions, options ...gitlab.RequestOptionFunc) (*gitlab.ProjectVariable, *gitlab.Response, error) {
						return &gitlab.ProjectVariable{Key: variableKey}, &gitlab.Response{}, nil
					},
				},
				cr: variable(
					withDefaultValues(),
				),
			},
			want: want{
				cr: variable(
					withDefaultValues(),
					withConditions(xpv1.Creating()),
				),
				result: managed.ExternalCreation{},
			},
		},
		"FailedCreation": {
			args: args{
				variable: &fake.MockClient{
					MockCreateProjectVariable: func(pid interface{}, opt *gitlab.CreateProjectVariableOptions, options ...gitlab.RequestOptionFunc) (*gitlab.ProjectVariable, *gitlab.Response, error) {
						return &gitlab.ProjectVariable{}, &gitlab.Response{}, errBoom
					},
				},
				cr: variable(
					withDefaultValues(),
				),
			},
			want: want{
				cr: variable(
					withDefaultValues(),
					withConditions(xpv1.Creating()),
				),
				err: errors.Wrap(errBoom, errCreateFailed),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{kube: tc.kube, client: tc.variable}
			o, err := e.Create(context.Background(), tc.args.cr)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.cr, tc.args.cr, test.EquateConditions()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.result, o); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	type want struct {
		cr     *v1alpha1.Variable
		result managed.ExternalUpdate
		err    error
	}

	cases := map[string]struct {
		args
		want
	}{
		"SuccessfulEditProject": {
			args: args{
				variable: &fake.MockClient{
					MockUpdateProjectVariable: func(pid interface{}, key string, opt *gitlab.UpdateProjectVariableOptions, options ...gitlab.RequestOptionFunc) (*gitlab.ProjectVariable, *gitlab.Response, error) {
						return &gitlab.ProjectVariable{}, &gitlab.Response{}, nil
					},
				},
				cr: variable(
					withKey(variableKey),
					withProjectID(projectID),
				),
			},
			want: want{
				cr: variable(
					withKey(variableKey),
					withProjectID(projectID),
				),
			},
		},
		"FailedEdit": {
			args: args{
				variable: &fake.MockClient{
					MockUpdateProjectVariable: func(pid interface{}, key string, opt *gitlab.UpdateProjectVariableOptions, options ...gitlab.RequestOptionFunc) (*gitlab.ProjectVariable, *gitlab.Response, error) {
						return &gitlab.ProjectVariable{}, &gitlab.Response{}, errBoom
					},
				},
				cr: variable(
					withKey(variableKey),
					withProjectID(projectID),
				),
			},
			want: want{
				cr: variable(
					withKey(variableKey),
					withProjectID(projectID),
				),
				err: errors.Wrap(errBoom, errUpdateFailed),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{kube: tc.kube, client: tc.variable}
			o, err := e.Update(context.Background(), tc.args.cr)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.cr, tc.args.cr, test.EquateConditions()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.result, o); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}
func TestDelete(t *testing.T) {
	type want struct {
		cr  *v1alpha1.Variable
		err error
	}

	cases := map[string]struct {
		args
		want
	}{
		"SuccessfulDeletion": {
			args: args{
				variable: &fake.MockClient{
					MockRemoveProjectVariable: func(pid interface{}, key string, options ...gitlab.RequestOptionFunc) (*gitlab.Response, error) {
						return &gitlab.Response{}, nil
					},
				},
				cr: variable(
					withProjectID(projectID),
					withConditions(xpv1.Available()),
				),
			},
			want: want{
				cr: variable(
					withProjectID(projectID),
					withConditions(xpv1.Deleting()),
				),
			},
		},
		"FailedDeletion": {
			args: args{
				variable: &fake.MockClient{
					MockRemoveProjectVariable: func(pid interface{}, key string, options ...gitlab.RequestOptionFunc) (*gitlab.Response, error) {
						return &gitlab.Response{}, errBoom
					},
				},
				cr: variable(
					withProjectID(projectID),
					withConditions(xpv1.Available()),
				),
			},
			want: want{
				cr: variable(
					withProjectID(projectID),
					withConditions(xpv1.Deleting()),
				),
				err: errors.Wrap(errBoom, errDeleteFailed),
			},
		},
		"InvalidVariableID": {
			args: args{
				variable: &fake.MockClient{
					MockRemoveProjectVariable: func(pid interface{}, key string, options ...gitlab.RequestOptionFunc) (*gitlab.Response, error) {
						return &gitlab.Response{}, nil
					},
				},
				cr: variable(
					withProjectID(projectID),
					withConditions(xpv1.Available()),
				),
			},
			want: want{
				cr: variable(
					withProjectID(projectID),
					withConditions(xpv1.Deleting()),
				),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{client: tc.variable}
			err := e.Delete(context.Background(), tc.args.cr)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.cr, tc.args.cr, test.EquateConditions()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}
