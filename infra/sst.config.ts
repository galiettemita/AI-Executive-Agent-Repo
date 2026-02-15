import { SSTConfig } from "sst";

export default {
  config(_input) {
    return {
      name: "executive-os",
      region: "us-east-1",
    };
  },
  async stacks(app) {
    const { NetworkStack } = await import("./stacks/NetworkStack");
    const { DataStack } = await import("./stacks/DataStack");
    const { EcsStack } = await import("./stacks/EcsStack");

    app.stack(NetworkStack);
    app.stack(DataStack);
    app.stack(EcsStack);
  },
} satisfies SSTConfig;
