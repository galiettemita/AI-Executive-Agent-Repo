export interface PdfToolsInput {
  action: 'extract_text' | 'merge' | 'split';
  files: string[];
  page_range?: string;
  output_name?: string;
}

export interface PdfToolsOutput {
  provider: 'pdf-tools';
  action: PdfToolsInput['action'];
  output_path: string;
  pages_processed: number;
  extracted_text_preview?: string;
}
