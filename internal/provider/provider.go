// Package provider implements the Cozystack Terraform/OpenTofu provider.
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

// Ensure CozystackProvider satisfies the provider.Provider interface.
var _ provider.Provider = (*CozystackProvider)(nil)

// CozystackProvider is the provider implementation.
type CozystackProvider struct {
	// version is set at build time and surfaced in the provider metadata.
	version string
}

// providerModel maps the provider configuration block to Go types.
type providerModel struct {
	Host                 types.String `tfsdk:"host"`
	Token                types.String `tfsdk:"token"`
	ClusterCACertificate types.String `tfsdk:"cluster_ca_certificate"`
	Insecure             types.Bool   `tfsdk:"insecure"`
	ConfigPath           types.String `tfsdk:"config_path"`
	ConfigContext        types.String `tfsdk:"config_context"`
	InCluster            types.Bool   `tfsdk:"in_cluster"`
}

// New returns a factory for the provider, wired with the build version.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &CozystackProvider{version: version}
	}
}

// Metadata sets the provider type name and version.
func (p *CozystackProvider) Metadata(
	_ context.Context,
	_ provider.MetadataRequest,
	resp *provider.MetadataResponse,
) {
	resp.TypeName = "cozystack"
	resp.Version = p.version
}

// Schema defines the provider-level configuration, mirroring the conventions of
// the official kubernetes provider so connection settings transfer directly.
func (p *CozystackProvider) Schema(
	_ context.Context,
	_ provider.SchemaRequest,
	resp *provider.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage Cozystack resources through its aggregated " +
			"Kubernetes API (`apps.cozystack.io`).",
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Kubernetes API server URL. Falls back to `KUBE_HOST`.",
			},
			"token": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Bearer token for authentication. Falls back to `KUBE_TOKEN`.",
			},
			"cluster_ca_certificate": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "PEM-encoded CA bundle used to verify the API server. " +
					"Falls back to `KUBE_CLUSTER_CA_CERT_DATA`.",
			},
			"insecure": schema.BoolAttribute{
				Optional: true,
				MarkdownDescription: "Skip TLS verification of the API server certificate. " +
					"Falls back to `KUBE_INSECURE`.",
			},
			"config_path": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Path to a kubeconfig file. " +
					"Falls back to `KUBE_CONFIG_PATH`, then `KUBECONFIG`.",
			},
			"config_context": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "kubeconfig context to use. Falls back to `KUBE_CTX`.",
			},
			"in_cluster": schema.BoolAttribute{
				Optional: true,
				MarkdownDescription: "Use the in-cluster service account configuration " +
					"instead of a kubeconfig.",
			},
		},
	}
}

// Configure resolves the connection settings and builds the API client shared
// by all resources and data sources.
func (p *CozystackProvider) Configure(
	ctx context.Context,
	req provider.ConfigureRequest,
	resp *provider.ConfigureResponse,
) {
	var model providerModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)

	if resp.Diagnostics.HasError() {
		return
	}

	conn := model.connectionConfig()

	conn.ApplyEnvDefaults()

	restConfig, err := conn.RestConfig()
	if err != nil {
		resp.Diagnostics.AddError("Unable to build Kubernetes client configuration", err.Error())

		return
	}

	api, err := client.NewForConfig(restConfig)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Cozystack API client", err.Error())

		return
	}

	resp.ResourceData = api
	resp.DataSourceData = api
}

// connectionConfig converts the Terraform model into a client.Config.
func (m *providerModel) connectionConfig() client.Config {
	return client.Config{
		Host:                 m.Host.ValueString(),
		Token:                m.Token.ValueString(),
		ClusterCACertificate: m.ClusterCACertificate.ValueString(),
		Insecure:             m.Insecure.ValueBool(),
		ConfigPath:           m.ConfigPath.ValueString(),
		ConfigContext:        m.ConfigContext.ValueString(),
		InCluster:            m.InCluster.ValueBool(),
	}
}

// Resources returns the resource types implemented by the provider.
func (p *CozystackProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewTenantResource,
		NewRedisResource,
		NewQdrantResource,
		NewBucketResource,
		newAppResource[openbaoResourceModel, *openbaoResourceModel](client.OpenBaoResource(), "openbao", openbaoSchema),
		newAppResource[vpnResourceModel, *vpnResourceModel](client.VPNResource(), "vpn", vpnSchema),
		newAppResource[rabbitmqResourceModel, *rabbitmqResourceModel](client.RabbitMQResource(), "rabbitmq", rabbitmqSchema),
		newAppResource[mariadbResourceModel, *mariadbResourceModel](client.MariaDBResource(), "mariadb", mariadbSchema),
		newAppResource[mongodbResourceModel, *mongodbResourceModel](client.MongoDBResource(), "mongodb", mongodbSchema),
		newAppResource[clickhouseResourceModel, *clickhouseResourceModel](client.ClickHouseResource(), "clickhouse", clickhouseSchema),
		newAppResource[natsResourceModel, *natsResourceModel](client.NATSResource(), "nats", natsSchema),
		newAppResource[opensearchResourceModel, *opensearchResourceModel](client.OpenSearchResource(), "opensearch", opensearchSchema),
		newAppResource[postgresqlResourceModel, *postgresqlResourceModel](client.PostgresResource(), "postgres", postgresSchema),
		newAppResource[httpcacheResourceModel, *httpcacheResourceModel](client.HTTPCacheResource(), "httpcache", httpcacheSchema),
		newAppResource[tcpbalancerResourceModel, *tcpbalancerResourceModel](client.TCPBalancerResource(), "tcpbalancer", tcpbalancerSchema),
		newAppResource[harborResourceModel, *harborResourceModel](client.HarborResource(), "harbor", harborSchema),
		newAppResource[vpcResourceModel, *vpcResourceModel](client.VPCResource(), "vpc", vpcSchema),
		newAppResource[vmdiskResourceModel, *vmdiskResourceModel](client.VMDiskResource(), "vmdisk", vmdiskSchema),
		newAppResource[kafkaResourceModel, *kafkaResourceModel](client.KafkaResource(), "kafka", kafkaSchema),
		newAppResource[foundationdbResourceModel, *foundationdbResourceModel](client.FoundationDBResource(), "foundationdb", foundationdbSchema),
		newAppResource[vminstanceResourceModel, *vminstanceResourceModel](client.VMInstanceResource(), "vminstance", vminstanceSchema),
	}
}

// DataSources returns the data source types implemented by the provider.
func (p *CozystackProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewTenantDataSource,
		NewRedisDataSource,
		NewQdrantDataSource,
		NewBucketDataSource,
		newAppDataSource[openbaoModel, *openbaoModel](client.OpenBaoResource(), "openbao", openbaoDataSourceSchema),
		newAppDataSource[vpnModel, *vpnModel](client.VPNResource(), "vpn", vpnDataSourceSchema),
		newAppDataSource[rabbitmqModel, *rabbitmqModel](client.RabbitMQResource(), "rabbitmq", rabbitmqDataSourceSchema),
		newAppDataSource[mariadbModel, *mariadbModel](client.MariaDBResource(), "mariadb", mariadbDataSourceSchema),
		newAppDataSource[mongodbModel, *mongodbModel](client.MongoDBResource(), "mongodb", mongodbDataSourceSchema),
		newAppDataSource[clickhouseModel, *clickhouseModel](client.ClickHouseResource(), "clickhouse", clickhouseDataSourceSchema),
		newAppDataSource[natsModel, *natsModel](client.NATSResource(), "nats", natsDataSourceSchema),
		newAppDataSource[opensearchModel, *opensearchModel](client.OpenSearchResource(), "opensearch", opensearchDataSourceSchema),
		newAppDataSource[postgresqlModel, *postgresqlModel](client.PostgresResource(), "postgres", postgresDataSourceSchema),
		newAppDataSource[httpcacheModel, *httpcacheModel](client.HTTPCacheResource(), "httpcache", httpcacheDataSourceSchema),
		newAppDataSource[tcpbalancerModel, *tcpbalancerModel](client.TCPBalancerResource(), "tcpbalancer", tcpbalancerDataSourceSchema),
		newAppDataSource[harborModel, *harborModel](client.HarborResource(), "harbor", harborDataSourceSchema),
		newAppDataSource[vpcModel, *vpcModel](client.VPCResource(), "vpc", vpcDataSourceSchema),
		newAppDataSource[vmdiskModel, *vmdiskModel](client.VMDiskResource(), "vmdisk", vmdiskDataSourceSchema),
		newAppDataSource[kafkaModel, *kafkaModel](client.KafkaResource(), "kafka", kafkaDataSourceSchema),
		newAppDataSource[foundationdbModel, *foundationdbModel](client.FoundationDBResource(), "foundationdb", foundationdbDataSourceSchema),
		newAppDataSource[vminstanceModel, *vminstanceModel](client.VMInstanceResource(), "vminstance", vminstanceDataSourceSchema),
	}
}

// providerClient extracts the configured API client from provider data,
// recording a diagnostic if the type is unexpected. It is shared by the
// Configure methods of resources and data sources.
func providerClient(providerData any, diags *diag.Diagnostics) *client.Client {
	if providerData == nil {
		return nil
	}

	api, ok := providerData.(*client.Client)
	if !ok {
		diags.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("expected *client.Client, got %T", providerData),
		)

		return nil
	}

	return api
}
