package resources

import (
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestCheckOwnership(t *testing.T) {
	type check func(t2 *testing.T, err error)

	hasNoErr := func() check {
		return func(t2 *testing.T, err error) {
			require.NoError(t2, err)
		}
	}

	hasAlreadyExistsErr := func() check {
		return func(t2 *testing.T, err error) {
			require.IsType(t2, &trace.AlreadyExistsError{}, err.(*trace.TraceErr).OrigError())
		}
	}

	validateCondition := func(t2 *testing.T, status metav1.ConditionStatus, reason string) func(condition metav1.Condition) error {
		return func(condition metav1.Condition) error {
			require.Equal(t2, condition.Type, ConditionTypeTeleportResourceOwned)
			require.Equal(t2, condition.Status, status)
			require.Equal(t2, condition.Reason, reason)
			return nil
		}
	}

	tests := []struct {
		name                    string
		existingResource        types.Resource
		expectedConditionStatus metav1.ConditionStatus
		expectedConditionReason string
		check                   check
	}{
		{
			name:                    "new resource",
			existingResource:        nil,
			expectedConditionStatus: metav1.ConditionTrue,
			expectedConditionReason: ConditionReasonNewResource,
			check:                   hasNoErr(),
		},
		{
			name: "existing owned resource",
			existingResource: &types.UserV2{
				Metadata: types.Metadata{
					Name:   "existing owned user",
					Labels: map[string]string{types.OriginLabel: types.OriginKubernetes},
				},
			},
			expectedConditionStatus: metav1.ConditionTrue,
			expectedConditionReason: ConditionReasonOriginLabelMatching,
			check:                   hasNoErr(),
		},
		{
			name: "existing unowned resource (no label)",
			existingResource: &types.UserV2{
				Metadata: types.Metadata{
					Name: "existing unowned user without label",
				},
			},
			expectedConditionStatus: metav1.ConditionFalse,
			expectedConditionReason: ConditionReasonOriginLabelNotMatching,
			check:                   hasAlreadyExistsErr(),
		},
		{
			name: "existing unowned resource (bad origin)",
			existingResource: &types.UserV2{
				Metadata: types.Metadata{
					Name:   "existing owned user without origin label",
					Labels: map[string]string{types.OriginLabel: types.OriginConfigFile},
				},
			},
			expectedConditionStatus: metav1.ConditionFalse,
			expectedConditionReason: ConditionReasonOriginLabelNotMatching,
			check:                   hasAlreadyExistsErr(),
		},
		{
			name: "existing unowned resource (no origin)",
			existingResource: &types.UserV2{
				Metadata: types.Metadata{
					Name:   "existing owned user without origin label",
					Labels: map[string]string{"foo": "bar"},
				},
			},
			expectedConditionStatus: metav1.ConditionFalse,
			expectedConditionReason: ConditionReasonOriginLabelNotMatching,
			check:                   hasAlreadyExistsErr(),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			err := checkOwnership(
				tc.existingResource,
				validateCondition(t, tc.expectedConditionStatus, tc.expectedConditionReason),
			)
			tc.check(t, err)
		})
	}
}
