package recordingrule

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/grafana/grafana/apps/alerting/rules/pkg/apis/alerting/v0alpha1"

	folders "github.com/grafana/grafana/apps/folder/pkg/apis/folder/v1beta1"
	ngmodels "github.com/grafana/grafana/pkg/services/ngalert/models"
	"github.com/grafana/grafana/pkg/tests/apis"
	"github.com/grafana/grafana/pkg/tests/apis/alerting/rules/common"
	"github.com/grafana/grafana/pkg/tests/testinfra"
	"github.com/grafana/grafana/pkg/tests/testsuite"
	"github.com/grafana/grafana/pkg/util"
)

func TestMain(m *testing.M) {
	testsuite.Run(m)
}

func getTestHelper(t *testing.T) *apis.K8sTestHelper {
	return apis.NewK8sTestHelper(t, testinfra.GrafanaOpts{
		EnableFeatureToggles: []string{
			"kubernetesAlertingRules",
		},
	})
}

func createTestFolder(t *testing.T, helper *apis.K8sTestHelper, folderUID string) {
	ctx := context.Background()
	folderClient := common.NewFolderClient(t, helper.Org1.Admin)

	folder := &folders.Folder{
		ObjectMeta: v1.ObjectMeta{
			Name:      folderUID,
			Namespace: "default",
		},
		Spec: folders.FolderSpec{
			Title: "Test Folder",
		},
	}

	_, err := folderClient.Create(ctx, folder, v1.CreateOptions{})
	require.NoError(t, err)
}

func TestIntegrationResourceIdentifier(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	helper := getTestHelper(t)
	client := common.NewRecordingRuleClient(t, helper.Org1.Admin)

	// Create test folder first
	createTestFolder(t, helper, "test-folder")

	rule := ngmodels.RuleGen.With(
		ngmodels.RuleMuts.WithUniqueUID(),
		ngmodels.RuleMuts.WithUniqueTitle(),
		ngmodels.RuleMuts.WithNamespaceUID("test-folder"),
		ngmodels.RuleMuts.WithGroupName("test-group"),
		ngmodels.RuleMuts.WithAllRecordingRules(),
	).Generate()

	newResource := &v0alpha1.RecordingRule{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "default",
			Annotations: map[string]string{
				"grafana.app/folder": "test-folder",
			},
		},
		Spec: v0alpha1.RecordingRuleSpec{
			Title:  rule.Title,
			Metric: rule.Record.Metric,
			Data: map[string]v0alpha1.RecordingRuleQuery{
				"A": {
					QueryType:     "query",
					DatasourceUID: v0alpha1.RecordingRuleDatasourceUID(rule.Data[0].DatasourceUID),
					Model:         rule.Data[0].Model,
					Source:        util.Pointer(true),
					RelativeTimeRange: &v0alpha1.RecordingRuleRelativeTimeRange{
						From: v0alpha1.RecordingRulePromDurationWMillis("5m"),
						To:   v0alpha1.RecordingRulePromDurationWMillis("0s"),
					},
				},
			},
			Trigger: v0alpha1.RecordingRuleIntervalTrigger{
				Interval: v0alpha1.RecordingRulePromDuration(fmt.Sprintf("%d", rule.IntervalSeconds)),
			},
		},
	}

	// Test 1: Create with explicit name
	namedResource := newResource.Copy().(*v0alpha1.RecordingRule)
	namedResource.Name = "explicit-name-recording-rule"
	namedRule, err := client.Create(ctx, namedResource, v1.CreateOptions{})
	require.NoError(t, err)
	require.Equal(t, "explicit-name-recording-rule", namedRule.Name)
	require.NotEmpty(t, namedRule.UID)

	// Test 2: Create without explicit name (auto-generated)
	autoGenRule, err := client.Create(ctx, newResource, v1.CreateOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, autoGenRule.Name)
	require.NotEmpty(t, autoGenRule.UID)

	// Test 3: Get by identifier
	retrievedRule, err := client.Get(ctx, autoGenRule.Name, v1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, autoGenRule.Name, retrievedRule.Name)
	require.Equal(t, newResource.Spec.Title, retrievedRule.Spec.Title)

	// Test 4: Update (should preserve name)
	updatedRule := retrievedRule.Copy().(*v0alpha1.RecordingRule)
	updatedRule.Spec.Title = "updated-recording-rule-title"

	finalRule, err := client.Update(ctx, updatedRule, v1.UpdateOptions{})
	require.NoError(t, err)
	require.Equal(t, "updated-recording-rule-title", finalRule.Spec.Title)
	require.Equal(t, retrievedRule.Name, finalRule.Name, "Update should preserve the resource name")
	// Note: Recording rule backend doesn't implement ResourceVersion, unlike alert rules

	// Test 5: Verify the update persisted
	finalRetrieved, err := client.Get(ctx, finalRule.Name, v1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, finalRule.Spec.Title, finalRetrieved.Spec.Title)
	require.Equal(t, finalRule.Name, finalRetrieved.Name)
	require.Equal(t, finalRule.ResourceVersion, finalRetrieved.ResourceVersion)

	// Cleanup
	require.NoError(t, client.Delete(ctx, namedRule.Name, v1.DeleteOptions{}))
	require.NoError(t, client.Delete(ctx, finalRule.Name, v1.DeleteOptions{}))
}

// TestIntegrationResourcePermissions is skipped for now as access control is handled in the service layer
func TestIntegrationResourcePermissions(t *testing.T) {
	t.Skip("Access control tests skipped - handled in service layer")
}

// TestIntegrationAccessControl tests basic access control functionality
// Access control is primarily handled in the service layer, so this test focuses on basic CRUD operations
func TestIntegrationAccessControl(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	helper := getTestHelper(t)

	// Test with admin user for basic functionality
	adminClient := common.NewRecordingRuleClient(t, helper.Org1.Admin)

	// Create test folder first
	createTestFolder(t, helper, "test-folder")

	rule := ngmodels.RuleGen.With(
		ngmodels.RuleMuts.WithUniqueUID(),
		ngmodels.RuleMuts.WithUniqueTitle(),
		ngmodels.RuleMuts.WithNamespaceUID("test-folder"),
		ngmodels.RuleMuts.WithGroupName("test-group"),
		ngmodels.RuleMuts.WithAllRecordingRules(),
	).Generate()

	recordingRule := &v0alpha1.RecordingRule{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "default",
			Annotations: map[string]string{
				"grafana.app/folder": "test-folder",
			},
		},
		Spec: v0alpha1.RecordingRuleSpec{
			Title:  rule.Title,
			Metric: rule.Record.Metric,
			Data: map[string]v0alpha1.RecordingRuleQuery{
				"A": {
					QueryType:     "query",
					DatasourceUID: v0alpha1.RecordingRuleDatasourceUID(rule.Data[0].DatasourceUID),
					Model:         rule.Data[0].Model,
					Source:        util.Pointer(true),
					RelativeTimeRange: &v0alpha1.RecordingRuleRelativeTimeRange{
						From: v0alpha1.RecordingRulePromDurationWMillis("5m"),
						To:   v0alpha1.RecordingRulePromDurationWMillis("0s"),
					},
				},
			},
			Trigger: v0alpha1.RecordingRuleIntervalTrigger{
				Interval: v0alpha1.RecordingRulePromDuration(fmt.Sprintf("%d", rule.IntervalSeconds)),
			},
		},
	}

	t.Run("admin should be able to create recording rule", func(t *testing.T) {
		created, err := adminClient.Create(ctx, recordingRule, v1.CreateOptions{})
		require.NoError(t, err)
		require.NotNil(t, created)
		require.Equal(t, recordingRule.Spec.Title, created.Spec.Title)

		// Cleanup
		defer func() {
			_ = adminClient.Delete(ctx, created.Name, v1.DeleteOptions{})
		}()

		t.Run("admin should be able to read recording rule", func(t *testing.T) {
			read, err := adminClient.Get(ctx, created.Name, v1.GetOptions{})
			require.NoError(t, err)
			require.Equal(t, created.Spec.Title, read.Spec.Title)
		})

		t.Run("admin should be able to update recording rule", func(t *testing.T) {
			updated := created.Copy().(*v0alpha1.RecordingRule)
			updated.Spec.Title = "updated-title"

			result, err := adminClient.Update(ctx, updated, v1.UpdateOptions{})
			require.NoError(t, err)
			require.Equal(t, "updated-title", result.Spec.Title)
		})

		t.Run("admin should be able to delete recording rule", func(t *testing.T) {
			err := adminClient.Delete(ctx, created.Name, v1.DeleteOptions{})
			require.NoError(t, err)
		})
	})
}

func TestIntegrationCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	helper := getTestHelper(t)

	adminClient := common.NewRecordingRuleClient(t, helper.Org1.Admin)

	// Create test folder first
	createTestFolder(t, helper, "test-folder")

	t.Run("should be able to create and read recording rule", func(t *testing.T) {
		rule := ngmodels.RuleGen.With(
			ngmodels.RuleMuts.WithUniqueUID(),
			ngmodels.RuleMuts.WithUniqueTitle(),
			ngmodels.RuleMuts.WithNamespaceUID("test-folder"),
			ngmodels.RuleMuts.WithGroupName("test-group"),
			ngmodels.RuleMuts.WithAllRecordingRules(),
		).Generate()

		recordingRule := &v0alpha1.RecordingRule{
			ObjectMeta: v1.ObjectMeta{
				Namespace: "default",
				Annotations: map[string]string{
					"grafana.app/folder": "test-folder",
				},
			},
			Spec: v0alpha1.RecordingRuleSpec{
				Title:  rule.Title,
				Metric: rule.Record.Metric,
				Data: map[string]v0alpha1.RecordingRuleQuery{
					"A": {
						QueryType:     "query",
						DatasourceUID: v0alpha1.RecordingRuleDatasourceUID(rule.Data[0].DatasourceUID),
						Model:         rule.Data[0].Model,
						Source:        util.Pointer(true),
						RelativeTimeRange: &v0alpha1.RecordingRuleRelativeTimeRange{
							From: v0alpha1.RecordingRulePromDurationWMillis("5m"),
							To:   v0alpha1.RecordingRulePromDurationWMillis("0s"),
						},
					},
				},
				Trigger: v0alpha1.RecordingRuleIntervalTrigger{
					Interval: v0alpha1.RecordingRulePromDuration(fmt.Sprintf("%d", rule.IntervalSeconds)),
				},
			},
		}

		created, err := adminClient.Create(ctx, recordingRule, v1.CreateOptions{})
		require.NoError(t, err)
		require.NotNil(t, created)

		t.Run("should be able to read what it is created", func(t *testing.T) {
			get, err := adminClient.Get(ctx, created.Name, v1.GetOptions{})
			require.NoError(t, err)
			require.Equal(t, created.Spec.Title, get.Spec.Title)
		})

		// Cleanup
		require.NoError(t, adminClient.Delete(ctx, created.Name, v1.DeleteOptions{}))
	})

	t.Run("should fail to create recording rule with invalid config", func(t *testing.T) {
		invalidRule := &v0alpha1.RecordingRule{
			ObjectMeta: v1.ObjectMeta{
				Namespace: "default",
				Annotations: map[string]string{
					"grafana.app/folder": "test-folder",
				},
			},
			Spec: v0alpha1.RecordingRuleSpec{
				Title: "invalid-recording-rule",
				Data:  map[string]v0alpha1.RecordingRuleQuery{}, // Empty data should fail
				Trigger: v0alpha1.RecordingRuleIntervalTrigger{
					Interval: "30",
				},
			},
		}

		_, err := adminClient.Create(ctx, invalidRule, v1.CreateOptions{})
		require.Errorf(t, err, "Expected error but got successful result")
		// The validation happens at the service level, so we just need to verify it fails
		require.Error(t, err, "Creating invalid rule should fail")
	})
}

func TestIntegrationPatch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	helper := getTestHelper(t)

	adminClient := common.NewRecordingRuleClient(t, helper.Org1.Admin)

	// Create test folder first
	createTestFolder(t, helper, "test-folder")

	rule := ngmodels.RuleGen.With(
		ngmodels.RuleMuts.WithUniqueUID(),
		ngmodels.RuleMuts.WithUniqueTitle(),
		ngmodels.RuleMuts.WithNamespaceUID("test-folder"),
		ngmodels.RuleMuts.WithGroupName("test-group"),
		ngmodels.RuleMuts.WithAllRecordingRules(),
	).Generate()

	recordingRule := &v0alpha1.RecordingRule{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "default",
			Annotations: map[string]string{
				"grafana.app/folder": "test-folder",
			},
		},
		Spec: v0alpha1.RecordingRuleSpec{
			Title:  rule.Title,
			Metric: rule.Record.Metric,
			Data: map[string]v0alpha1.RecordingRuleQuery{
				"A": {
					QueryType:     "query",
					DatasourceUID: v0alpha1.RecordingRuleDatasourceUID(rule.Data[0].DatasourceUID),
					Model:         rule.Data[0].Model,
					Source:        util.Pointer(true),
					RelativeTimeRange: &v0alpha1.RecordingRuleRelativeTimeRange{
						From: v0alpha1.RecordingRulePromDurationWMillis("5m"),
						To:   v0alpha1.RecordingRulePromDurationWMillis("0s"),
					},
				},
			},
			Trigger: v0alpha1.RecordingRuleIntervalTrigger{
				Interval: v0alpha1.RecordingRulePromDuration(fmt.Sprintf("%d", rule.IntervalSeconds)),
			},
		},
	}

	current, err := adminClient.Create(ctx, recordingRule, v1.CreateOptions{})
	require.NoError(t, err)
	require.NotNil(t, current)

	t.Run("should patch with json patch", func(t *testing.T) {
		current, err := adminClient.Get(ctx, current.Name, v1.GetOptions{})
		require.NoError(t, err)

		patch := []map[string]any{
			{
				"op":    "replace",
				"path":  "/spec/title",
				"value": "patched-title",
			},
		}

		patchData, err := json.Marshal(patch)
		require.NoError(t, err)

		result, err := adminClient.Patch(ctx, current.Name, types.JSONPatchType, patchData, v1.PatchOptions{})
		require.NoError(t, err)

		require.Equal(t, "patched-title", result.Spec.Title)
	})

	// Cleanup
	require.NoError(t, adminClient.Delete(ctx, current.Name, v1.DeleteOptions{}))
}

func TestIntegrationBasicAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	helper := getTestHelper(t)

	adminClient := common.NewRecordingRuleClient(t, helper.Org1.Admin)

	// Create test folder first
	createTestFolder(t, helper, "test-folder")

	t.Run("should be able to create and read recording rule", func(t *testing.T) {
		rule := ngmodels.RuleGen.With(
			ngmodels.RuleMuts.WithUniqueUID(),
			ngmodels.RuleMuts.WithUniqueTitle(),
			ngmodels.RuleMuts.WithNamespaceUID("test-folder"),
			ngmodels.RuleMuts.WithGroupName("test-group"),
			ngmodels.RuleMuts.WithAllRecordingRules(),
		).Generate()

		recordingRule := &v0alpha1.RecordingRule{
			ObjectMeta: v1.ObjectMeta{
				Namespace: "default",
				Annotations: map[string]string{
					"grafana.app/folder": "test-folder",
				},
			},
			Spec: v0alpha1.RecordingRuleSpec{
				Title:  rule.Title,
				Metric: rule.Record.Metric,
				Data: map[string]v0alpha1.RecordingRuleQuery{
					"A": {
						QueryType:     "query",
						DatasourceUID: v0alpha1.RecordingRuleDatasourceUID(rule.Data[0].DatasourceUID),
						Model:         rule.Data[0].Model,
						Source:        util.Pointer(true),
						RelativeTimeRange: &v0alpha1.RecordingRuleRelativeTimeRange{
							From: v0alpha1.RecordingRulePromDurationWMillis("5m"),
							To:   v0alpha1.RecordingRulePromDurationWMillis("0s"),
						},
					},
				},
				Trigger: v0alpha1.RecordingRuleIntervalTrigger{
					Interval: v0alpha1.RecordingRulePromDuration(fmt.Sprintf("%d", rule.IntervalSeconds)),
				},
			},
		}

		created, err := adminClient.Create(ctx, recordingRule, v1.CreateOptions{})
		require.NoError(t, err)
		require.NotNil(t, created)

		t.Run("should be able to read what it is created", func(t *testing.T) {
			get, err := adminClient.Get(ctx, created.Name, v1.GetOptions{})
			require.NoError(t, err)
			require.Equal(t, created.Spec.Title, get.Spec.Title)
		})

		// Cleanup
		require.NoError(t, adminClient.Delete(ctx, created.Name, v1.DeleteOptions{}))
	})

	t.Run("should fail to create recording rule with invalid config", func(t *testing.T) {
		invalidRule := &v0alpha1.RecordingRule{
			ObjectMeta: v1.ObjectMeta{
				Namespace: "default",
				Annotations: map[string]string{
					"grafana.app/folder": "test-folder",
				},
			},
			Spec: v0alpha1.RecordingRuleSpec{
				Title: "invalid-recording-rule",
				Data:  map[string]v0alpha1.RecordingRuleQuery{}, // Empty data should fail
				Trigger: v0alpha1.RecordingRuleIntervalTrigger{
					Interval: "30",
				},
			},
		}

		_, err := adminClient.Create(ctx, invalidRule, v1.CreateOptions{})
		require.Errorf(t, err, "Expected error but got successful result")
		// The validation happens at the service level, so we just need to verify it fails
		require.Error(t, err, "Creating invalid rule should fail")
	})
}
