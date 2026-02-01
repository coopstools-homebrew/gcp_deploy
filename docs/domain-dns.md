# Linking your personal domain to the VM

Use a subdomain of your domain (e.g. **ssh.coopstools.com**) so you can SSH to a hostname instead of an IP.

## How to get the VM's IP (for DNS)

- **Locally:** After `pulumi up`, run: `pulumi stack output externalIP`
- **From GitHub Actions:** The IP is printed in the Pulumi step output / run summary.

The IP is stable because we use a reserved static external IP in Pulumi.

## Create an A record in your domain manager

In whatever service manages DNS for your domain (e.g. your registrar or a DNS host), create an **A record** that points your chosen subdomain (e.g. `ssh` for ssh.coopstools.com) to the IP from above.

- **Type:** A  
- **Host/Name:** the subdomain (e.g. `ssh`)  
- **Value/Points to:** the VM's external IP  

After DNS propagates, you can SSH with:

```bash
ssh YOUR_USER@your-subdomain.your-domain.com
```

Replace with your SSH username and chosen subdomain/domain.

## If the VM's IP ever changes

If you recreate the static IP in Pulumi, update the A record in your domain manager to the new IP and wait for propagation again.
