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

// applicationTeardownTimeout bounds how long CheckDestroy waits for the async
// HelmRelease teardown behind a deleted application to complete.
const applicationTeardownTimeout = 5 * time.Minute

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

func newAccClient() (*client.Client, error) {
	conn := client.Config{}
	conn.ApplyEnvDefaults()

	restConfig, err := conn.RestConfig()
	if err != nil {
		return nil, fmt.Errorf("building rest config: %w", err)
	}

	api, err := client.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("building client: %w", err)
	}

	return api, nil
}

// checkApplicationDestroy returns a CheckDestroy that waits for every resource of
// the given Terraform type to disappear from the cluster.
func checkApplicationDestroy(res client.Resource, tfType string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		api, err := newAccClient()
		if err != nil {
			return err
		}

		for _, rs := range state.RootModule().Resources {
			if rs.Type != tfType {
				continue
			}

			if err := waitApplicationGone(api, res, rs.Primary.Attributes["namespace"], rs.Primary.Attributes["name"]); err != nil {
				return err
			}
		}

		return nil
	}
}

// waitApplicationGone polls until the application disappears, tolerating the
// asynchronous HelmRelease teardown behind a deleted application.
func waitApplicationGone(api *client.Client, res client.Resource, namespace, name string) error {
	deadline := time.Now().Add(applicationTeardownTimeout)

	for {
		app, err := api.Get(context.Background(), res, namespace, name)
		if client.IsNotFound(err) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("checking %s %s/%s: %w", res.Kind, namespace, name, err)
		}

		// The provider issued the delete (now terminating); the remaining teardown
		// is the platform's async work, not the resource's correctness.
		if app.Deleting {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("%s %s/%s still exists after %s", res.Kind, namespace, name, applicationTeardownTimeout)
		}

		time.Sleep(3 * time.Second)
	}
}

func TestAccTenantResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkApplicationDestroy(client.TenantResource(), "cozystack_tenant"),
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
		CheckDestroy:             checkApplicationDestroy(client.TenantResource(), "cozystack_tenant"),
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

func testAccRedisConfigBasic(name string) string {
	return fmt.Sprintf(`
resource "cozystack_redis" "test" {
  name      = %[1]q
  namespace = "tenant-root"
  replicas  = 1
}
`, name)
}

func testAccRedisConfigPreset(name string) string {
	return fmt.Sprintf(`
resource "cozystack_redis" "test" {
  name             = %[1]q
  namespace        = "tenant-root"
  replicas         = 1
  resources_preset = "t1.micro"
}
`, name)
}

func TestAccRedisResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkApplicationDestroy(client.RedisResource(), "cozystack_redis"),
		Steps: []resource.TestStep{
			{
				Config: testAccRedisConfigBasic("tfaccredis"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cozystack_redis.test", "name", "tfaccredis"),
					resource.TestCheckResourceAttr("cozystack_redis.test", "namespace", "tenant-root"),
					resource.TestCheckResourceAttr("cozystack_redis.test", "version", "v8"),
					resource.TestCheckResourceAttr("cozystack_redis.test", "replicas", "1"),
					resource.TestCheckResourceAttr("cozystack_redis.test", "id", "tenant-root/tfaccredis"),
				),
			},
			{
				ResourceName:            "cozystack_redis.test",
				ImportState:             true,
				ImportStateId:           "tenant-root/tfaccredis",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"wait_for_ready", "wait_timeout", "ready", "chart_version"},
			},
			{
				Config: testAccRedisConfigPreset("tfaccredis"),
				Check: resource.TestCheckResourceAttr(
					"cozystack_redis.test", "resources_preset", "t1.micro",
				),
			},
		},
	})
}

func TestAccRedisDataSource(t *testing.T) {
	config := testAccRedisConfigBasic("tfaccredisds") + `
data "cozystack_redis" "test" {
  name      = cozystack_redis.test.name
  namespace = cozystack_redis.test.namespace
}
`

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkApplicationDestroy(client.RedisResource(), "cozystack_redis"),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.cozystack_redis.test", "name",
						"cozystack_redis.test", "name",
					),
					resource.TestCheckResourceAttr("data.cozystack_redis.test", "version", "v8"),
					resource.TestCheckResourceAttrSet("data.cozystack_redis.test", "id"),
				),
			},
		},
	})
}

func testAccQdrantConfigBasic(name string) string {
	return fmt.Sprintf(`
resource "cozystack_qdrant" "test" {
  name      = %[1]q
  namespace = "tenant-root"
  replicas  = 1
}
`, name)
}

func testAccQdrantConfigPreset(name string) string {
	return fmt.Sprintf(`
resource "cozystack_qdrant" "test" {
  name             = %[1]q
  namespace        = "tenant-root"
  replicas         = 1
  resources_preset = "t1.medium"
}
`, name)
}

func TestAccQdrantResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkApplicationDestroy(client.QdrantResource(), "cozystack_qdrant"),
		Steps: []resource.TestStep{
			{
				Config: testAccQdrantConfigBasic("tfaccqdrant"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cozystack_qdrant.test", "name", "tfaccqdrant"),
					resource.TestCheckResourceAttr("cozystack_qdrant.test", "namespace", "tenant-root"),
					resource.TestCheckResourceAttr("cozystack_qdrant.test", "replicas", "1"),
					resource.TestCheckResourceAttr("cozystack_qdrant.test", "id", "tenant-root/tfaccqdrant"),
				),
			},
			{
				ResourceName:            "cozystack_qdrant.test",
				ImportState:             true,
				ImportStateId:           "tenant-root/tfaccqdrant",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"wait_for_ready", "wait_timeout", "ready", "chart_version"},
			},
			{
				Config: testAccQdrantConfigPreset("tfaccqdrant"),
				Check: resource.TestCheckResourceAttr(
					"cozystack_qdrant.test", "resources_preset", "t1.medium",
				),
			},
		},
	})
}

func TestAccQdrantDataSource(t *testing.T) {
	config := testAccQdrantConfigBasic("tfaccqdrantds") + `
data "cozystack_qdrant" "test" {
  name      = cozystack_qdrant.test.name
  namespace = cozystack_qdrant.test.namespace
}
`

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkApplicationDestroy(client.QdrantResource(), "cozystack_qdrant"),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.cozystack_qdrant.test", "name",
						"cozystack_qdrant.test", "name",
					),
					resource.TestCheckResourceAttrSet("data.cozystack_qdrant.test", "id"),
				),
			},
		},
	})
}

func testAccBucketConfigOneUser(name string) string {
	return fmt.Sprintf(`
resource "cozystack_bucket" "test" {
  name      = %[1]q
  namespace = "tenant-root"
  users = {
    reader = { readonly = true }
  }
}
`, name)
}

func testAccBucketConfigTwoUsers(name string) string {
	return fmt.Sprintf(`
resource "cozystack_bucket" "test" {
  name      = %[1]q
  namespace = "tenant-root"
  users = {
    reader = { readonly = true }
    writer = { readonly = false }
  }
}
`, name)
}

func TestAccBucketResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkApplicationDestroy(client.BucketResource(), "cozystack_bucket"),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketConfigOneUser("tfaccbucket"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cozystack_bucket.test", "name", "tfaccbucket"),
					resource.TestCheckResourceAttr("cozystack_bucket.test", "id", "tenant-root/tfaccbucket"),
					resource.TestCheckResourceAttr("cozystack_bucket.test", "users.reader.readonly", "true"),
				),
			},
			{
				ResourceName:            "cozystack_bucket.test",
				ImportState:             true,
				ImportStateId:           "tenant-root/tfaccbucket",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"wait_for_ready", "wait_timeout", "ready", "chart_version"},
			},
			{
				Config: testAccBucketConfigTwoUsers("tfaccbucket"),
				Check: resource.TestCheckResourceAttr(
					"cozystack_bucket.test", "users.writer.readonly", "false",
				),
			},
		},
	})
}

func testAccOpenbaoConfig(name string, ui bool) string {
	return fmt.Sprintf(`
resource "cozystack_openbao" "test" {
  name      = %[1]q
  namespace = "tenant-root"
  ui        = %[2]t
}
`, name, ui)
}

func TestAccOpenbaoResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkApplicationDestroy(client.OpenBaoResource(), "cozystack_openbao"),
		Steps: []resource.TestStep{
			{
				Config: testAccOpenbaoConfig("tfaccvault", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cozystack_openbao.test", "name", "tfaccvault"),
					resource.TestCheckResourceAttr("cozystack_openbao.test", "ui", "true"),
					resource.TestCheckResourceAttr("cozystack_openbao.test", "id", "tenant-root/tfaccvault"),
				),
			},
			{
				ResourceName:            "cozystack_openbao.test",
				ImportState:             true,
				ImportStateId:           "tenant-root/tfaccvault",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"wait_for_ready", "wait_timeout", "ready", "chart_version"},
			},
			{
				Config: testAccOpenbaoConfig("tfaccvault", false),
				Check:  resource.TestCheckResourceAttr("cozystack_openbao.test", "ui", "false"),
			},
		},
	})
}

func testAccVPNConfig(name string, users string) string {
	return fmt.Sprintf(`
resource "cozystack_vpn" "test" {
  name      = %[1]q
  namespace = "tenant-root"
  users = {
%[2]s
  }
}
`, name, users)
}

func TestAccVPNResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkApplicationDestroy(client.VPNResource(), "cozystack_vpn"),
		Steps: []resource.TestStep{
			{
				Config: testAccVPNConfig("tfaccvpn", `    alice = { password = "test-password-123" }`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cozystack_vpn.test", "name", "tfaccvpn"),
					resource.TestCheckResourceAttr("cozystack_vpn.test", "id", "tenant-root/tfaccvpn"),
					resource.TestCheckResourceAttr("cozystack_vpn.test", "users.alice.password", "test-password-123"),
				),
			},
			{
				ResourceName:            "cozystack_vpn.test",
				ImportState:             true,
				ImportStateId:           "tenant-root/tfaccvpn",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"wait_for_ready", "wait_timeout", "ready", "chart_version"},
			},
			{
				Config: testAccVPNConfig("tfaccvpn", "    alice = { password = \"test-password-123\" }\n    bob = { password = \"another-pass-456\" }"),
				Check:  resource.TestCheckResourceAttr("cozystack_vpn.test", "users.bob.password", "another-pass-456"),
			},
		},
	})
}

func TestAccRabbitmqResource(t *testing.T) {
	base := `
resource "cozystack_rabbitmq" "test" {
  name      = "tfaccrabbit"
  namespace = "tenant-root"
  replicas  = 1
  users     = { app = { password = "pw-123" } }
  vhosts    = { main = { roles = { admin = ["app"] } } }
}
`
	updated := `
resource "cozystack_rabbitmq" "test" {
  name      = "tfaccrabbit"
  namespace = "tenant-root"
  replicas  = 1
  users     = { app = { password = "pw-123" } }
  vhosts    = { main = { roles = { admin = ["app"], readonly = ["guest"] } } }
}
`

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkApplicationDestroy(client.RabbitMQResource(), "cozystack_rabbitmq"),
		Steps: []resource.TestStep{
			{
				Config: base,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cozystack_rabbitmq.test", "id", "tenant-root/tfaccrabbit"),
					resource.TestCheckResourceAttr("cozystack_rabbitmq.test", "users.app.password", "pw-123"),
					resource.TestCheckResourceAttr("cozystack_rabbitmq.test", "vhosts.main.roles.admin.0", "app"),
				),
			},
			{
				ResourceName:            "cozystack_rabbitmq.test",
				ImportState:             true,
				ImportStateId:           "tenant-root/tfaccrabbit",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"wait_for_ready", "wait_timeout", "ready", "chart_version"},
			},
			{
				Config: updated,
				Check:  resource.TestCheckResourceAttr("cozystack_rabbitmq.test", "vhosts.main.roles.readonly.0", "guest"),
			},
		},
	})
}

func TestAccMariadbResource(t *testing.T) {
	base := `
resource "cozystack_mariadb" "test" {
  name      = "tfaccmaria"
  namespace = "tenant-root"
  replicas  = 1
  users     = { app = { password = "pw-123" } }
  databases = { appdb = { roles = { admin = ["app"] } } }
}
`
	updated := `
resource "cozystack_mariadb" "test" {
  name      = "tfaccmaria"
  namespace = "tenant-root"
  replicas  = 1
  users     = { app = { password = "pw-123" }, ro = { password = "pw-456" } }
  databases = { appdb = { roles = { admin = ["app"], readonly = ["ro"] } } }
}
`

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkApplicationDestroy(client.MariaDBResource(), "cozystack_mariadb"),
		Steps: []resource.TestStep{
			{
				Config: base,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cozystack_mariadb.test", "id", "tenant-root/tfaccmaria"),
					resource.TestCheckResourceAttr("cozystack_mariadb.test", "users.app.password", "pw-123"),
					resource.TestCheckResourceAttr("cozystack_mariadb.test", "databases.appdb.roles.admin.0", "app"),
				),
			},
			{
				ResourceName:            "cozystack_mariadb.test",
				ImportState:             true,
				ImportStateId:           "tenant-root/tfaccmaria",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"wait_for_ready", "wait_timeout", "ready", "chart_version"},
			},
			{
				Config: updated,
				Check:  resource.TestCheckResourceAttr("cozystack_mariadb.test", "databases.appdb.roles.readonly.0", "ro"),
			},
		},
	})
}

func TestAccMongodbResource(t *testing.T) {
	config := `
resource "cozystack_mongodb" "test" {
  name      = "tfaccmongo"
  namespace = "tenant-root"
  replicas  = 1
  users     = { app = { password = "pw-123" } }
  databases = { appdb = { roles = { admin = ["app"] } } }
}
`

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkApplicationDestroy(client.MongoDBResource(), "cozystack_mongodb"),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cozystack_mongodb.test", "id", "tenant-root/tfaccmongo"),
					resource.TestCheckResourceAttr("cozystack_mongodb.test", "sharding", "false"),
					resource.TestCheckResourceAttr("cozystack_mongodb.test", "users.app.password", "pw-123"),
				),
			},
			{
				ResourceName:            "cozystack_mongodb.test",
				ImportState:             true,
				ImportStateId:           "tenant-root/tfaccmongo",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"wait_for_ready", "wait_timeout", "ready", "chart_version"},
			},
		},
	})
}

func TestAccBucketDataSource(t *testing.T) {
	config := testAccBucketConfigOneUser("tfaccbucketds") + `
data "cozystack_bucket" "test" {
  name      = cozystack_bucket.test.name
  namespace = cozystack_bucket.test.namespace
}
`

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkApplicationDestroy(client.BucketResource(), "cozystack_bucket"),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.cozystack_bucket.test", "name",
						"cozystack_bucket.test", "name",
					),
					resource.TestCheckResourceAttr("data.cozystack_bucket.test", "users.reader.readonly", "true"),
				),
			},
		},
	})
}

func TestAccClickhouseResource(t *testing.T) {
	config := `
resource "cozystack_clickhouse" "test" {
  name      = "tfaccch"
  namespace = "tenant-root"
  replicas  = 1
  users     = { reader = { password = "pw-123", readonly = true } }
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkApplicationDestroy(client.ClickHouseResource(), "cozystack_clickhouse"),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cozystack_clickhouse.test", "id", "tenant-root/tfaccch"),
					resource.TestCheckResourceAttr("cozystack_clickhouse.test", "users.reader.readonly", "true"),
				),
			},
			{
				ResourceName:            "cozystack_clickhouse.test",
				ImportState:             true,
				ImportStateId:           "tenant-root/tfaccch",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"wait_for_ready", "wait_timeout", "ready", "chart_version"},
			},
		},
	})
}

func TestAccPostgresResource(t *testing.T) {
	config := `
resource "cozystack_postgres" "test" {
  name      = "tfaccpg"
  namespace = "tenant-root"
  replicas  = 1
  users     = { app = { password = "pw-123" } }
  databases = { appdb = { roles = { admin = ["app"] } } }
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkApplicationDestroy(client.PostgresResource(), "cozystack_postgres"),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cozystack_postgres.test", "id", "tenant-root/tfaccpg"),
					resource.TestCheckResourceAttr("cozystack_postgres.test", "users.app.password", "pw-123"),
					resource.TestCheckResourceAttr("cozystack_postgres.test", "databases.appdb.roles.admin.0", "app"),
				),
			},
			{
				ResourceName:            "cozystack_postgres.test",
				ImportState:             true,
				ImportStateId:           "tenant-root/tfaccpg",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"wait_for_ready", "wait_timeout", "ready", "chart_version"},
			},
		},
	})
}

func TestAccHTTPCacheResource(t *testing.T) {
	config := `
resource "cozystack_httpcache" "test" {
  name      = "tfacchc"
  namespace = "tenant-root"
  size      = "1Gi"
  endpoints = ["192.0.2.10:80"]
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkApplicationDestroy(client.HTTPCacheResource(), "cozystack_httpcache"),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cozystack_httpcache.test", "id", "tenant-root/tfacchc"),
					resource.TestCheckResourceAttr("cozystack_httpcache.test", "endpoints.0", "192.0.2.10:80"),
				),
			},
			{
				ResourceName:            "cozystack_httpcache.test",
				ImportState:             true,
				ImportStateId:           "tenant-root/tfacchc",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"wait_for_ready", "wait_timeout", "ready", "chart_version"},
			},
		},
	})
}

func TestAccTCPBalancerResource(t *testing.T) {
	config := `
resource "cozystack_tcpbalancer" "test" {
  name           = "tfacclb"
  namespace      = "tenant-root"
  replicas       = 1
  whitelist_http = true
  whitelist      = ["192.0.2.0/24"]
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkApplicationDestroy(client.TCPBalancerResource(), "cozystack_tcpbalancer"),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cozystack_tcpbalancer.test", "id", "tenant-root/tfacclb"),
					resource.TestCheckResourceAttr("cozystack_tcpbalancer.test", "whitelist.0", "192.0.2.0/24"),
				),
			},
			{
				ResourceName:            "cozystack_tcpbalancer.test",
				ImportState:             true,
				ImportStateId:           "tenant-root/tfacclb",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"wait_for_ready", "wait_timeout", "ready", "chart_version"},
			},
		},
	})
}

func TestAccHarborResource(t *testing.T) {
	config := `
resource "cozystack_harbor" "test" {
  name      = "tfacchb"
  namespace = "tenant-root"
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkApplicationDestroy(client.HarborResource(), "cozystack_harbor"),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cozystack_harbor.test", "id", "tenant-root/tfacchb"),
				),
			},
			{
				ResourceName:            "cozystack_harbor.test",
				ImportState:             true,
				ImportStateId:           "tenant-root/tfacchb",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"wait_for_ready", "wait_timeout", "ready", "chart_version"},
			},
		},
	})
}

func TestAccVPCResource(t *testing.T) {
	config := `
resource "cozystack_vpc" "test" {
  name      = "tfaccvpc"
  namespace = "tenant-root"
  subnets = [
    { name = "web", cidr = "10.0.0.0/24" },
  ]
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkApplicationDestroy(client.VPCResource(), "cozystack_vpc"),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cozystack_vpc.test", "id", "tenant-root/tfaccvpc"),
					resource.TestCheckResourceAttr("cozystack_vpc.test", "subnets.0.name", "web"),
				),
			},
			{
				ResourceName:            "cozystack_vpc.test",
				ImportState:             true,
				ImportStateId:           "tenant-root/tfaccvpc",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"wait_for_ready", "wait_timeout", "ready", "chart_version"},
			},
		},
	})
}
