export type ExcalidrawFlowchartAction = 'generate_flowchart' | 'update_flowchart' | 'export_svg';

export interface ExcalidrawFlowchartInput {
  action: ExcalidrawFlowchartAction;
  description?: string;
  flowchart_id?: string;
  nodes?: string[];
}

export interface ExcalidrawNode {
  id: string;
  label: string;
}

export interface ExcalidrawEdge {
  from: string;
  to: string;
}

export interface ExcalidrawFlowchartOutput {
  provider: 'excalidraw-flowchart';
  action: ExcalidrawFlowchartAction;
  flowchart_id: string;
  nodes: ExcalidrawNode[];
  edges: ExcalidrawEdge[];
  export_url?: string;
  summary: string;
}
