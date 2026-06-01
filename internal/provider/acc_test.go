package provider_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
	"github.com/lexfrei/terraform-provider-cozystack/internal/provider"
)

// tenantTeardownTimeout bounds how long CheckDestroy waits for the async
// HelmRelease teardown behind a deleted tenant to complete.
const tenantTeardownTimeout = 3 * time.Minute

// testAccProtoV6ProviderFactories registers the in-process provider server used
// by the acceptance tests.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"cozystack": providerserver.NewProtocol6WithError(provider.New("test")()),
}

// testAccPreCheck skips acceptance tests unless a kubeconfig is available. The
// cluster must already run Cozystack; the cluster name is taken from the
// environment and never hard-coded.
func testAccPreCheck(t *testing.T) {
	t.Helper()

	if os.Getenv("KUBECONFIG") == "" && os.Getenv("KUBE_CONFIG_PATH") == "" {
		t.Skip("set KUBECONFIG (and optionally KUBE_CTX) to run acceptance tests")
	}
}

// testAccTenantConfigBasic is a minimal tenant: every infrastructure flag stays
// off, so create and teardown are quick and reliable.
func testAccTenantConfigBasic(name string) string {
	return fmt.Sprintf(`
resource "cozystack_tenant" "test" {
  name      = %[1]q
  namespace = "tenant-root"
}
`, name)
}

// testAccTenantConfigQuotas exercises a spec update with a lightweight field
// (resource quotas) rather than one that deploys an infrastructure stack.
func testAccTenantConfigQuotas(name string) string {
	return fmt.Sprintf(`
resource "cozystack_tenant" "test" {
  name      = %[1]q
  namespace = "tenant-root"
  resource_quotas = {
    cpu = "4"
  }
}
`, name)
}

func testAccCheckTenantDestroy(state *terraform.State) error {
	conn := client.Config{}
	conn.ApplyEnvDefaults()

	restConfig, err := conn.RestConfig()
	if err != nil {
		return fmt.Errorf("building rest config: %w", err)
	}

	api, err := client.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("building client: %w", err)
	}

	for _, rs := range state.RootModule().Resources {
		if rs.Type != "cozystack_tenant" {
			continue
		}

		if err := waitTenantGone(api, rs.Primary.Attributes["namespace"], rs.Primary.Attributes["name"]); err != nil {
			return err
		}
	}

	return nil
}

// waitTenantGone polls until the tenant disappears, tolerating the asynchronous
// HelmRelease teardown behind a deleted tenant.
func waitTenantGone(api *client.Client, namespace, name string) error {
	deadline := time.Now().Add(tenantTeardownTimeout)

	for {
		_, err := api.GetTenant(context.Background(), namespace, name)
		if client.IsNotFound(err) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("checking tenant %s/%s: %w", namespace, name, err)
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("tenant %s/%s still exists after %s", namespace, name, tenantTeardownTimeout)
		}

		time.Sleep(3 * time.Second)
	}
}

func TestAccTenantResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckTenantDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccTenantConfigBasic("tfacc"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cozystack_tenant.test", "name", "tfacc"),
					resource.TestCheckResourceAttr("cozystack_tenant.test", "namespace", "tenant-root"),
					resource.TestCheckResourceAttr("cozystack_tenant.test", "monitoring", "false"),
					resource.TestCheckResourceAttr("cozystack_tenant.test", "id", "tenant-root/tfacc"),
					resource.TestCheckResourceAttrSet("cozystack_tenant.test", "status_namespace"),
				),
			},
			{
				ResourceName:      "cozystack_tenant.test",
				ImportState:       true,
				ImportStateId:     "tenant-root/tfacc",
				ImportStateVerify: true,
				// ready and version are eventually-consistent status fields that
				// can advance between create and import.
				ImportStateVerifyIgnore: []string{"wait_for_ready", "wait_timeout", "ready", "version"},
			},
			{
				Config: testAccTenantConfigQuotas("tfacc"),
				Check: resource.TestCheckResourceAttr(
					"cozystack_tenant.test", "resource_quotas.cpu", "4",
				),
			},
		},
	})
}

func TestAccTenantDataSource(t *testing.T) {
	config := testAccTenantConfigBasic("tfaccds") + `
data "cozystack_tenant" "test" {
  name      = cozystack_tenant.test.name
  namespace = cozystack_tenant.test.namespace
}
`

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckTenantDestroy,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.cozystack_tenant.test", "name",
						"cozystack_tenant.test", "name",
					),
					resource.TestCheckResourceAttr("data.cozystack_tenant.test", "namespace", "tenant-root"),
					resource.TestCheckResourceAttrSet("data.cozystack_tenant.test", "id"),
				),
			},
		},
	})
}
