import { StackContext, use } from "sst/constructs";
import * as ecs from "aws-cdk-lib/aws-ecs";
import * as ec2 from "aws-cdk-lib/aws-ec2";
import * as ecr from "aws-cdk-lib/aws-ecr";
import * as elbv2 from "aws-cdk-lib/aws-elasticloadbalancingv2";
import * as secretsmanager from "aws-cdk-lib/aws-secretsmanager";
import * as logs from "aws-cdk-lib/aws-logs";
import { SecretValue, Duration } from "aws-cdk-lib";
import { NetworkStack } from "./NetworkStack";
import { DataStack } from "./DataStack";

export function EcsStack({ stack }: StackContext) {
  const { vpc, alb, appSecurityGroup } = use(NetworkStack);
  const { database, redis, attachments, knowledgeSnapshots, voice, documents } = use(DataStack);

  const cluster = new ecs.Cluster(stack, "ExecutiveCluster", {
    vpc,
    containerInsights: true,
  });
  // Internal service discovery for plane-to-plane calls (VPC-only).
  cluster.addDefaultCloudMapNamespace({ name: "executive-os.local" });

  const gatewayRepo = new ecr.Repository(stack, "GatewayRepo");
  const brainRepo = new ecr.Repository(stack, "BrainRepo");
  const handsRepo = new ecr.Repository(stack, "HandsRepo");
  const workersRepo = new ecr.Repository(stack, "WorkersRepo");

  const stageEnv = stack.stage === "prod" ? "production" : "staging";

  const appSecrets =
    stack.stage === "dev"
      ? new secretsmanager.Secret(stack, "AppSecrets", {
          secretName: `executive-os/${stack.stage}/app`,
          secretObjectValue: {
            OPENAI_API_KEY: SecretValue.unsafePlainText("CHANGEME"),
            WA_ACCESS_TOKEN: SecretValue.unsafePlainText("CHANGEME"),
            WA_VERIFY_TOKEN: SecretValue.unsafePlainText("CHANGEME"),
            WA_APP_SECRET: SecretValue.unsafePlainText("CHANGEME"),
            WA_PHONE_NUMBER_ID: SecretValue.unsafePlainText("CHANGEME"),
            WA_BUSINESS_ACCOUNT_ID: SecretValue.unsafePlainText("CHANGEME"),
            CLERK_SECRET_KEY: SecretValue.unsafePlainText("CHANGEME"),
            GOOGLE_CLIENT_ID: SecretValue.unsafePlainText("CHANGEME"),
            GOOGLE_CLIENT_SECRET: SecretValue.unsafePlainText("CHANGEME"),
            GOOGLE_REDIRECT_URI: SecretValue.unsafePlainText(""),
            MS_CLIENT_ID: SecretValue.unsafePlainText("CHANGEME"),
            MS_CLIENT_SECRET: SecretValue.unsafePlainText("CHANGEME"),
            MS_REDIRECT_URI: SecretValue.unsafePlainText(""),
            MS_TENANT_ID: SecretValue.unsafePlainText("common"),
            MICROSOFT_CLIENT_ID: SecretValue.unsafePlainText("CHANGEME"),
            MICROSOFT_CLIENT_SECRET: SecretValue.unsafePlainText("CHANGEME"),
            TAVILY_API_KEY: SecretValue.unsafePlainText("CHANGEME"),
            ANTHROPIC_API_KEY: SecretValue.unsafePlainText("CHANGEME"),
            GOOGLE_AI_API_KEY: SecretValue.unsafePlainText("CHANGEME"),
            LOCAL_LLM_ENDPOINT: SecretValue.unsafePlainText(""),
            LLM_ROUTER_FAILOVER_TIMEOUT_S: SecretValue.unsafePlainText("30"),
            LLM_ROUTER_HEALTH_CHECK_INTERVAL: SecretValue.unsafePlainText("30"),
            STRIPE_SECRET_KEY: SecretValue.unsafePlainText("CHANGEME"),
            SENTRY_DSN: SecretValue.unsafePlainText("CHANGEME"),
            AXIOM_API_TOKEN: SecretValue.unsafePlainText("CHANGEME"),
            DATABASE_URL: SecretValue.unsafePlainText("CHANGEME"),
            APP_BASE_URL: SecretValue.unsafePlainText("CHANGEME"),
            JWT_SECRET: SecretValue.unsafePlainText("CHANGEME"),
            PII_ENCRYPTION_KEY: SecretValue.unsafePlainText("CHANGEME"),
            OTEL_EXPORTER_OTLP_ENDPOINT: SecretValue.unsafePlainText(""),
            OTEL_EXPORTER_OTLP_HEADERS: SecretValue.unsafePlainText(""),
            OTEL_EXPORTER_OTLP_TRACES_ENDPOINT: SecretValue.unsafePlainText(""),
            OTEL_EXPORTER_OTLP_TRACES_HEADERS: SecretValue.unsafePlainText(""),
            OTEL_EXPORTER_OTLP_METRICS_ENDPOINT: SecretValue.unsafePlainText(""),
            OTEL_EXPORTER_OTLP_METRICS_HEADERS: SecretValue.unsafePlainText(""),
            OTEL_METRICS_ENABLED: SecretValue.unsafePlainText("0"),
            ENABLE_SCHEDULER: SecretValue.unsafePlainText("0"),
            EMAIL_PROVIDER: SecretValue.unsafePlainText("ses"),
            SES_REGION: SecretValue.unsafePlainText("us-east-1"),
            SES_CONFIGURATION_SET: SecretValue.unsafePlainText(""),
            FROM_EMAIL: SecretValue.unsafePlainText(""),
            FROM_NAME: SecretValue.unsafePlainText("Executive OS"),
            POSTHOG_API_KEY: SecretValue.unsafePlainText(""),
            POSTHOG_HOST: SecretValue.unsafePlainText("https://app.posthog.com"),
            FEATURE_MULTI_PROVIDER_LLM: SecretValue.unsafePlainText("true"),
            FEATURE_VOICE_INPUT: SecretValue.unsafePlainText("true"),
            FEATURE_IMAGE_PROCESSING: SecretValue.unsafePlainText("true"),
            FEATURE_DOCUMENT_PROCESSING: SecretValue.unsafePlainText("false"),
            FEATURE_PRIVILEGE_ISOLATION: SecretValue.unsafePlainText("false"),
            FEATURE_PROFILING_ENABLED: SecretValue.unsafePlainText("true"),
            FEATURE_CONSOLIDATION_ENABLED: SecretValue.unsafePlainText("false"),
            FEATURE_SELF_REVIEW_ENABLED: SecretValue.unsafePlainText("false"),
          },
        })
      : secretsmanager.Secret.fromSecretNameV2(
          stack,
          "AppSecrets",
          `executive-os/${stack.stage}/app`,
        );

  const logGroup = new logs.LogGroup(stack, "ExecutiveLogs", {
    retention: logs.RetentionDays.THIRTY_DAYS,
  });

  const baseEnv = {
    ENV: stageEnv,
    // ElastiCache has transit encryption enabled; use TLS.
    REDIS_URL: `rediss://${redis.attrPrimaryEndPointAddress}:6379/0?ssl_cert_reqs=required`,
    OTEL_ENABLED: "1",
    AWS_REGION: stack.region,
    S3_BUCKET: attachments.bucketName,
    S3_KNOWLEDGE_BUCKET: knowledgeSnapshots.bucketName,
    S3_VOICE_BUCKET: voice.bucketName,
    S3_DOCUMENTS_BUCKET: documents.bucketName,
    // Cloud Map names (brain.executive-os.local, hands.executive-os.local)
    BRAIN_INTERNAL_BASE_URL: "http://brain.executive-os.local:8000",
    HANDS_INTERNAL_BASE_URL: "http://hands.executive-os.local:8000",
  };

  const secretEnv = {
    OPENAI_API_KEY: ecs.Secret.fromSecretsManager(appSecrets, "OPENAI_API_KEY"),
    WA_ACCESS_TOKEN: ecs.Secret.fromSecretsManager(appSecrets, "WA_ACCESS_TOKEN"),
    WA_VERIFY_TOKEN: ecs.Secret.fromSecretsManager(appSecrets, "WA_VERIFY_TOKEN"),
    WA_APP_SECRET: ecs.Secret.fromSecretsManager(appSecrets, "WA_APP_SECRET"),
    WA_PHONE_NUMBER_ID: ecs.Secret.fromSecretsManager(appSecrets, "WA_PHONE_NUMBER_ID"),
    WA_BUSINESS_ACCOUNT_ID: ecs.Secret.fromSecretsManager(appSecrets, "WA_BUSINESS_ACCOUNT_ID"),
    CLERK_SECRET_KEY: ecs.Secret.fromSecretsManager(appSecrets, "CLERK_SECRET_KEY"),
    GOOGLE_CLIENT_ID: ecs.Secret.fromSecretsManager(appSecrets, "GOOGLE_CLIENT_ID"),
    GOOGLE_CLIENT_SECRET: ecs.Secret.fromSecretsManager(appSecrets, "GOOGLE_CLIENT_SECRET"),
    GOOGLE_REDIRECT_URI: ecs.Secret.fromSecretsManager(appSecrets, "GOOGLE_REDIRECT_URI"),
    MS_CLIENT_ID: ecs.Secret.fromSecretsManager(appSecrets, "MS_CLIENT_ID"),
    MS_CLIENT_SECRET: ecs.Secret.fromSecretsManager(appSecrets, "MS_CLIENT_SECRET"),
    MS_REDIRECT_URI: ecs.Secret.fromSecretsManager(appSecrets, "MS_REDIRECT_URI"),
    MS_TENANT_ID: ecs.Secret.fromSecretsManager(appSecrets, "MS_TENANT_ID"),
    MICROSOFT_CLIENT_ID: ecs.Secret.fromSecretsManager(appSecrets, "MICROSOFT_CLIENT_ID"),
    MICROSOFT_CLIENT_SECRET: ecs.Secret.fromSecretsManager(appSecrets, "MICROSOFT_CLIENT_SECRET"),
    TAVILY_API_KEY: ecs.Secret.fromSecretsManager(appSecrets, "TAVILY_API_KEY"),
    ANTHROPIC_API_KEY: ecs.Secret.fromSecretsManager(appSecrets, "ANTHROPIC_API_KEY"),
    GOOGLE_AI_API_KEY: ecs.Secret.fromSecretsManager(appSecrets, "GOOGLE_AI_API_KEY"),
    LOCAL_LLM_ENDPOINT: ecs.Secret.fromSecretsManager(appSecrets, "LOCAL_LLM_ENDPOINT"),
    LLM_ROUTER_FAILOVER_TIMEOUT_S: ecs.Secret.fromSecretsManager(appSecrets, "LLM_ROUTER_FAILOVER_TIMEOUT_S"),
    LLM_ROUTER_HEALTH_CHECK_INTERVAL: ecs.Secret.fromSecretsManager(appSecrets, "LLM_ROUTER_HEALTH_CHECK_INTERVAL"),
    STRIPE_SECRET_KEY: ecs.Secret.fromSecretsManager(appSecrets, "STRIPE_SECRET_KEY"),
    SENTRY_DSN: ecs.Secret.fromSecretsManager(appSecrets, "SENTRY_DSN"),
    AXIOM_API_TOKEN: ecs.Secret.fromSecretsManager(appSecrets, "AXIOM_API_TOKEN"),
    DATABASE_URL: ecs.Secret.fromSecretsManager(appSecrets, "DATABASE_URL"),
    APP_BASE_URL: ecs.Secret.fromSecretsManager(appSecrets, "APP_BASE_URL"),
    JWT_SECRET: ecs.Secret.fromSecretsManager(appSecrets, "JWT_SECRET"),
    PII_ENCRYPTION_KEY: ecs.Secret.fromSecretsManager(appSecrets, "PII_ENCRYPTION_KEY"),
    OTEL_EXPORTER_OTLP_ENDPOINT: ecs.Secret.fromSecretsManager(appSecrets, "OTEL_EXPORTER_OTLP_ENDPOINT"),
    OTEL_EXPORTER_OTLP_HEADERS: ecs.Secret.fromSecretsManager(appSecrets, "OTEL_EXPORTER_OTLP_HEADERS"),
    OTEL_EXPORTER_OTLP_TRACES_ENDPOINT: ecs.Secret.fromSecretsManager(appSecrets, "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"),
    OTEL_EXPORTER_OTLP_TRACES_HEADERS: ecs.Secret.fromSecretsManager(appSecrets, "OTEL_EXPORTER_OTLP_TRACES_HEADERS"),
    OTEL_EXPORTER_OTLP_METRICS_ENDPOINT: ecs.Secret.fromSecretsManager(appSecrets, "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"),
    OTEL_EXPORTER_OTLP_METRICS_HEADERS: ecs.Secret.fromSecretsManager(appSecrets, "OTEL_EXPORTER_OTLP_METRICS_HEADERS"),
    OTEL_METRICS_ENABLED: ecs.Secret.fromSecretsManager(appSecrets, "OTEL_METRICS_ENABLED"),
    ENABLE_SCHEDULER: ecs.Secret.fromSecretsManager(appSecrets, "ENABLE_SCHEDULER"),
    EMAIL_PROVIDER: ecs.Secret.fromSecretsManager(appSecrets, "EMAIL_PROVIDER"),
    SES_REGION: ecs.Secret.fromSecretsManager(appSecrets, "SES_REGION"),
    SES_CONFIGURATION_SET: ecs.Secret.fromSecretsManager(appSecrets, "SES_CONFIGURATION_SET"),
    FROM_EMAIL: ecs.Secret.fromSecretsManager(appSecrets, "FROM_EMAIL"),
    FROM_NAME: ecs.Secret.fromSecretsManager(appSecrets, "FROM_NAME"),
    POSTHOG_API_KEY: ecs.Secret.fromSecretsManager(appSecrets, "POSTHOG_API_KEY"),
    POSTHOG_HOST: ecs.Secret.fromSecretsManager(appSecrets, "POSTHOG_HOST"),
    FEATURE_MULTI_PROVIDER_LLM: ecs.Secret.fromSecretsManager(appSecrets, "FEATURE_MULTI_PROVIDER_LLM"),
    FEATURE_VOICE_INPUT: ecs.Secret.fromSecretsManager(appSecrets, "FEATURE_VOICE_INPUT"),
    FEATURE_IMAGE_PROCESSING: ecs.Secret.fromSecretsManager(appSecrets, "FEATURE_IMAGE_PROCESSING"),
    FEATURE_DOCUMENT_PROCESSING: ecs.Secret.fromSecretsManager(appSecrets, "FEATURE_DOCUMENT_PROCESSING"),
    FEATURE_PRIVILEGE_ISOLATION: ecs.Secret.fromSecretsManager(appSecrets, "FEATURE_PRIVILEGE_ISOLATION"),
    FEATURE_PROFILING_ENABLED: ecs.Secret.fromSecretsManager(appSecrets, "FEATURE_PROFILING_ENABLED"),
    FEATURE_CONSOLIDATION_ENABLED: ecs.Secret.fromSecretsManager(appSecrets, "FEATURE_CONSOLIDATION_ENABLED"),
    FEATURE_SELF_REVIEW_ENABLED: ecs.Secret.fromSecretsManager(appSecrets, "FEATURE_SELF_REVIEW_ENABLED"),
  };

  const gatewayTask = new ecs.FargateTaskDefinition(stack, "GatewayTask", {
    cpu: 512,
    memoryLimitMiB: 1024,
  });
  const gatewayContainer = gatewayTask.addContainer("GatewayContainer", {
    image: ecs.ContainerImage.fromEcrRepository(gatewayRepo, "latest"),
    logging: ecs.LogDriver.awsLogs({ logGroup, streamPrefix: "gateway" }),
    environment: { ...baseEnv, OTEL_SERVICE_NAME: "executive-os-gateway" },
    secrets: secretEnv,
    command: ["uvicorn", "app.planes.gateway_app:app", "--host", "0.0.0.0", "--port", "8000"],
    healthCheck: {
      command: ["CMD-SHELL", "curl -fsS http://localhost:8000/health || exit 1"],
      interval: Duration.seconds(30),
      timeout: Duration.seconds(5),
      retries: 3,
      startPeriod: Duration.seconds(30),
    },
  });
  gatewayContainer.addPortMappings({ containerPort: 8000 });

  const brainTask = new ecs.FargateTaskDefinition(stack, "BrainTask", {
    cpu: 1024,
    memoryLimitMiB: 2048,
  });
  const brainContainer = brainTask.addContainer("BrainContainer", {
    image: ecs.ContainerImage.fromEcrRepository(brainRepo, "latest"),
    logging: ecs.LogDriver.awsLogs({ logGroup, streamPrefix: "brain" }),
    environment: { ...baseEnv, OTEL_SERVICE_NAME: "executive-os-brain" },
    secrets: secretEnv,
    command: ["uvicorn", "app.planes.brain_app:app", "--host", "0.0.0.0", "--port", "8000"],
    healthCheck: {
      command: ["CMD-SHELL", "curl -fsS http://localhost:8000/health || exit 1"],
      interval: Duration.seconds(30),
      timeout: Duration.seconds(5),
      retries: 3,
      startPeriod: Duration.seconds(30),
    },
  });
  brainContainer.addPortMappings({ containerPort: 8000 });

  const handsTask = new ecs.FargateTaskDefinition(stack, "HandsTask", {
    cpu: 512,
    memoryLimitMiB: 1024,
  });
  const handsContainer = handsTask.addContainer("HandsContainer", {
    image: ecs.ContainerImage.fromEcrRepository(handsRepo, "latest"),
    logging: ecs.LogDriver.awsLogs({ logGroup, streamPrefix: "hands" }),
    environment: { ...baseEnv, OTEL_SERVICE_NAME: "executive-os-hands" },
    secrets: secretEnv,
    command: ["uvicorn", "app.planes.hands_app:app", "--host", "0.0.0.0", "--port", "8000"],
    healthCheck: {
      command: ["CMD-SHELL", "curl -fsS http://localhost:8000/health || exit 1"],
      interval: Duration.seconds(30),
      timeout: Duration.seconds(5),
      retries: 3,
      startPeriod: Duration.seconds(30),
    },
  });
  handsContainer.addPortMappings({ containerPort: 8000 });

  const workersTask = new ecs.FargateTaskDefinition(stack, "WorkersTask", {
    cpu: 512,
    memoryLimitMiB: 1024,
  });
  workersTask.addContainer("WorkersContainer", {
    image: ecs.ContainerImage.fromEcrRepository(workersRepo, "latest"),
    logging: ecs.LogDriver.awsLogs({ logGroup, streamPrefix: "workers" }),
    environment: { ...baseEnv, OTEL_SERVICE_NAME: "executive-os-workers" },
    secrets: secretEnv,
    // Run Celery workers (async processing, webhooks, background jobs)
    command: [
      "celery",
      "-A",
      "app.core.celery_app.celery_app",
      "worker",
      "-l",
      "info",
      "-Q",
      "default",
      "--concurrency",
      "2",
    ],
  });

  // Plane task roles need S3 access for attachments, knowledge snapshots, voice, and docs.
  for (const task of [gatewayTask, brainTask, handsTask, workersTask]) {
    attachments.grantReadWrite(task.taskRole);
    knowledgeSnapshots.grantReadWrite(task.taskRole);
    voice.grantReadWrite(task.taskRole);
    documents.grantReadWrite(task.taskRole);
  }

  const gatewayService = new ecs.FargateService(stack, "GatewayService", {
    cluster,
    taskDefinition: gatewayTask,
    desiredCount: 1,
    securityGroups: [appSecurityGroup],
    vpcSubnets: { subnetType: ec2.SubnetType.PRIVATE_WITH_EGRESS },
    healthCheckGracePeriod: Duration.seconds(60),
  });

  const brainService = new ecs.FargateService(stack, "BrainService", {
    cluster,
    taskDefinition: brainTask,
    desiredCount: 1,
    securityGroups: [appSecurityGroup],
    vpcSubnets: { subnetType: ec2.SubnetType.PRIVATE_WITH_EGRESS },
    cloudMapOptions: { name: "brain" },
  });

  const handsService = new ecs.FargateService(stack, "HandsService", {
    cluster,
    taskDefinition: handsTask,
    desiredCount: 1,
    securityGroups: [appSecurityGroup],
    vpcSubnets: { subnetType: ec2.SubnetType.PRIVATE_WITH_EGRESS },
    cloudMapOptions: { name: "hands" },
  });

  const workersService = new ecs.FargateService(stack, "WorkersService", {
    cluster,
    taskDefinition: workersTask,
    desiredCount: 1,
    securityGroups: [appSecurityGroup],
    vpcSubnets: { subnetType: ec2.SubnetType.PRIVATE_WITH_EGRESS },
  });

  const listener = alb.addListener("HttpListener", {
    port: 80,
    open: true,
  });

  const gatewayTargetGroup = listener.addTargets("GatewayTargets", {
    port: 80,
    targets: [gatewayService],
    healthCheck: {
      path: "/health",
      healthyHttpCodes: "200",
      interval: Duration.seconds(30),
    },
  });

  const certArn = (process.env.ALB_CERTIFICATE_ARN || "").trim();
  const importedHttpsListenerArn = (process.env.ALB_HTTPS_LISTENER_ARN || "").trim();
  const createHttpsListener = (process.env.ALB_CREATE_HTTPS_LISTENER || "").trim() === "1";

  // If a listener already exists (manually created or from a previous stack), import it
  // and skip listener creation to avoid CloudFormation AlreadyExists conflicts.
  if (createHttpsListener && certArn && !importedHttpsListenerArn) {
    alb.addListener("HttpsListener", {
      port: 443,
      protocol: elbv2.ApplicationProtocol.HTTPS,
      open: true,
      certificates: [elbv2.ListenerCertificate.fromArn(certArn)],
      defaultTargetGroups: [gatewayTargetGroup],
    });
  }

  stack.addOutputs({
    ClusterName: cluster.clusterName,
    GatewayRepo: gatewayRepo.repositoryUri,
    BrainRepo: brainRepo.repositoryUri,
    HandsRepo: handsRepo.repositoryUri,
    WorkersRepo: workersRepo.repositoryUri,
    AppSecretsArn: appSecrets.secretArn,
  });

  return {
    cluster,
    alb,
    appSecurityGroup,
    gatewayRepo,
    brainRepo,
    handsRepo,
    workersRepo,
    gatewayService,
    brainService,
    handsService,
    workersService,
  };
}
