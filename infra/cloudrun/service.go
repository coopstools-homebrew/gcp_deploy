package cloudrun

import (
	"fmt"

	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/cloudrun"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// CreateHelloService provisions the Cloud Run "hello" service and returns its URL output.
func CreateHelloService(ctx *pulumi.Context, project, region, image string) (pulumi.StringOutput, error) {
	svc, err := cloudrun.NewService(ctx, "hello", &cloudrun.ServiceArgs{
		Name:     pulumi.String("gcp-deploy-hello"),
		Location: pulumi.String(region),
		Project:  pulumi.String(project),
		Template: &cloudrun.ServiceTemplateArgs{
			Spec: &cloudrun.ServiceTemplateSpecArgs{
				Containers: cloudrun.ServiceTemplateSpecContainerArray{
					&cloudrun.ServiceTemplateSpecContainerArgs{
						Image: pulumi.String(image),
					},
				},
			},
		},
		Traffics: cloudrun.ServiceTrafficArray{
			&cloudrun.ServiceTrafficArgs{
				Percent:        pulumi.Int(100),
				LatestRevision: pulumi.Bool(true),
			},
		},
	})
	if err != nil {
		return pulumi.StringOutput{}, err
	}

	// Allow unauthenticated invokes (for Phase 1 test via curl/browser).
	if _, err = cloudrun.NewIamMember(ctx, "hello-invoker", &cloudrun.IamMemberArgs{
		Location: svc.Location,
		Project:  svc.Project,
		Service:  svc.Name,
		Role:     pulumi.String("roles/run.invoker"),
		Member:   pulumi.String("allUsers"),
	}); err != nil {
		return pulumi.StringOutput{}, err
	}

	url := svc.Statuses.ApplyT(func(statuses []cloudrun.ServiceStatus) (string, error) {
		if len(statuses) == 0 || statuses[0].Url == nil {
			return "", fmt.Errorf("no status yet")
		}
		return *statuses[0].Url, nil
	}).(pulumi.StringOutput)

	return url, nil
}

