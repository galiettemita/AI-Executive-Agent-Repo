import type { PdfToolsInput, PdfToolsOutput } from './types.js';

export async function runClient(input: PdfToolsInput): Promise<PdfToolsOutput> {
  const outputName = input.output_name ?? `${input.action}-output.pdf`;

  if (input.action === 'extract_text') {
    return {
      provider: 'pdf-tools',
      action: 'extract_text',
      output_path: `/tmp/${outputName}`,
      pages_processed: 4,
      extracted_text_preview: 'Executive summary: quarterly goals, staffing updates, and risk register.'
    };
  }

  if (input.action === 'merge') {
    return {
      provider: 'pdf-tools',
      action: 'merge',
      output_path: `/tmp/${outputName}`,
      pages_processed: input.files.length * 3
    };
  }

  return {
    provider: 'pdf-tools',
    action: 'split',
    output_path: `/tmp/${outputName}`,
    pages_processed: 2
  };
}
