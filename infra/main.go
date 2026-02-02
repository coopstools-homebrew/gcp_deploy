package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/cloudrun"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/compute"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/projects"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "gcp")
		project := cfg.Require("project")
		zone := cfg.Get("zone")
		if zone == "" {
			zone = "us-central1-a"
		}

		// SSH keys: config "sshKeys" (format "user:key") or read from ../ssh-keys/admin.pub with default user "james"
		sshKeys, err := getSSHKeys(ctx, cfg)
		if err != nil {
			return err
		}

		// Startup script: disable password auth, allow key-only SSH
		startupScript := `#!/bin/bash
sed -i 's/^#*PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config
sed -i 's/^#*PermitRootLogin.*/PermitRootLogin no/' /etc/ssh/sshd_config
systemctl restart sshd
`

		// Reserved static external IP (region = us-central1 for free tier)
		extIP, err := compute.NewAddress(ctx, "vm-ip", &compute.AddressArgs{
			Name:    pulumi.String("gcp-deploy-vm-ip"),
			Region:  pulumi.String("us-central1"),
			Project: pulumi.String(project),
		})
		if err != nil {
			return err
		}

		// Enable Cloud Run API (no manual step)
		_, err = projects.NewService(ctx, "run-api", &projects.ServiceArgs{
			Project: pulumi.String(project),
			Service: pulumi.String("run.googleapis.com"),
		})
		if err != nil {
			return err
		}

		// Firewall: allow SSH from anywhere (lock down later with your IP if desired)
		_, err = compute.NewFirewall(ctx, "allow-ssh", &compute.FirewallArgs{
			Name:    pulumi.String("allow-ssh-gcp-deploy"),
			Network: pulumi.String("default"),
			Project: pulumi.String(project),
			Allows: compute.FirewallAllowArray{
				&compute.FirewallAllowArgs{
					Protocol: pulumi.String("tcp"),
					Ports:    pulumi.StringArray{pulumi.String("22")},
				},
			},
			SourceRanges: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
		})
		if err != nil {
			return err
		}

		// Firewall: allow HTTP/HTTPS to VM (for Phase 3 reverse proxy)
		_, err = compute.NewFirewall(ctx, "allow-http-https", &compute.FirewallArgs{
			Name:    pulumi.String("allow-http-https-gcp-deploy"),
			Network: pulumi.String("default"),
			Project: pulumi.String(project),
			Allows: compute.FirewallAllowArray{
				&compute.FirewallAllowArgs{
					Protocol: pulumi.String("tcp"),
					Ports:    pulumi.StringArray{pulumi.String("80"), pulumi.String("443")},
				},
			},
			SourceRanges: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
		})
		if err != nil {
			return err
		}

		// e2-micro instance with static IP, SSH keys, startup script
		instance, err := compute.NewInstance(ctx, "vm", &compute.InstanceArgs{
			Name:        pulumi.String("gcp-deploy-vm"),
			MachineType: pulumi.String("e2-micro"),
			Zone:        pulumi.String(zone),
			Project:     pulumi.String(project),
			BootDisk: &compute.InstanceBootDiskArgs{
				InitializeParams: &compute.InstanceBootDiskInitializeParamsArgs{
					Image: pulumi.String("debian-cloud/debian-12"),
					Size:  pulumi.Int(10),
				},
			},
			NetworkInterfaces: compute.InstanceNetworkInterfaceArray{
				&compute.InstanceNetworkInterfaceArgs{
					Network: pulumi.String("default"),
					AccessConfigs: compute.InstanceNetworkInterfaceAccessConfigArray{
						&compute.InstanceNetworkInterfaceAccessConfigArgs{
							NatIp: extIP.Address,
						},
					},
				},
			},
			Metadata: pulumi.StringMap{
				"ssh-keys":       pulumi.String(sshKeys),
				"startup-script": pulumi.String(startupScript),
			},
		})
		if err != nil {
			return err
		}

		// Phase 1: Cloud Run service (placeholder image, min 0, us-central1)
		cloudRunSvc, err := cloudrun.NewService(ctx, "hello", &cloudrun.ServiceArgs{
			Name:     pulumi.String("gcp-deploy-hello"),
			Location: pulumi.String("us-central1"),
			Project:  pulumi.String(project),
			Template: &cloudrun.ServiceTemplateArgs{
				Spec: &cloudrun.ServiceTemplateSpecArgs{
					Containers: cloudrun.ServiceTemplateSpecContainerArray{
						&cloudrun.ServiceTemplateSpecContainerArgs{
							Image: pulumi.String("us-docker.pkg.dev/cloudrun/container/hello"),
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
			return err
		}

		// Allow unauthenticated invokes (for Phase 1 test via curl/browser)
		_, err = cloudrun.NewIamMember(ctx, "hello-invoker", &cloudrun.IamMemberArgs{
			Location: cloudRunSvc.Location,
			Project:  cloudRunSvc.Project,
			Service:  cloudRunSvc.Name,
			Role:     pulumi.String("roles/run.invoker"),
			Member:   pulumi.String("allUsers"),
		})
		if err != nil {
			return err
		}

		// Outputs
		ctx.Export("instanceName", instance.Name)
		ctx.Export("instanceId", instance.InstanceId)
		ctx.Export("externalIP", extIP.Address)
		sshCmd := extIP.Address.ApplyT(func(ip string) string {
			return fmt.Sprintf("ssh james@%s", ip)
		})
		ctx.Export("sshCommand", sshCmd)
		ctx.Export("cloudRunUrl", cloudRunSvc.Statuses.ApplyT(func(statuses []cloudrun.ServiceStatus) (string, error) {
			if len(statuses) == 0 || statuses[0].Url == nil {
				return "", fmt.Errorf("no status yet")
			}
			return *statuses[0].Url, nil
		}).(pulumi.StringOutput))
		return nil
	})
}

func getSSHKeys(ctx *pulumi.Context, cfg *config.Config) (string, error) {
	if keys := cfg.Get("sshKeys"); keys != "" {
		return keys, nil
	}
	// Try ../ssh-keys/admin.pub relative to infra/
	infraDir, err := os.Getwd()
	if err != nil {
		infraDir = "."
	}
	path := filepath.Join(infraDir, "..", "ssh-keys", "admin.pub")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("set config sshKeys (e.g. pulumi config set sshKeys 'user:ssh-rsa AAAA...') or add %s: %w", path, err)
	}
	line := string(data)
	// GCP format is "username:key". If line has no ":", prepend default user.
	if len(line) > 0 && line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
	}
	for i := 0; i < len(line); i++ {
		if line[i] == ' ' {
			// "ssh-rsa AAAA..." or "ecdsa-sha2-... AAAA..." -> no leading user; use james
			return "james:" + line, nil
		}
		if line[i] == ':' {
			return line, nil
		}
	}
	return "james:" + line, nil
}
