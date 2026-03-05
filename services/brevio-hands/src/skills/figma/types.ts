export type FigmaAction = 'analyze_file' | 'export_asset' | 'audit_accessibility';

export interface FigmaInput {
  action: FigmaAction;
  file_key?: string;
  node_id?: string;
  format?: 'png' | 'svg' | 'pdf';
}

export interface FigmaOutput {
  provider: 'figma';
  action: FigmaAction;
  file_key: string;
  findings: string[];
  export_url?: string;
  summary: string;
}
