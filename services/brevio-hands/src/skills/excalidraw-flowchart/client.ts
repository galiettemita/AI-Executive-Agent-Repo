import type { ExcalidrawFlowchartInput, ExcalidrawFlowchartOutput } from './types.js';

export async function runClient(input: ExcalidrawFlowchartInput): Promise<ExcalidrawFlowchartOutput> {
  const flowchartID = input.flowchart_id ?? 'flow-onboarding';
  const nodeLabels = input.nodes ?? ['Start', 'Review', 'Ship'];
  const nodes = nodeLabels.map((label, index) => ({ id: `n-${index + 1}`, label }));
  const edges = nodes.slice(1).map((node, index) => ({ from: nodes[index]?.id ?? 'n-1', to: node.id }));

  return {
    provider: 'excalidraw-flowchart',
    action: input.action,
    flowchart_id: flowchartID,
    nodes,
    edges,
    export_url: input.action === 'export_svg' ? `https://assets.brevio.local/${flowchartID}.svg` : undefined,
    summary: `Excalidraw flowchart ${flowchartID} processed with ${nodes.length} nodes.`
  };
}
