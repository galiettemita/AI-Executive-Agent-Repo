import type { ChromecastDevice, ChromecastInput, ChromecastOutput } from './types.js';

const BASE_DEVICES: ChromecastDevice[] = [
  {
    device_name: 'Living Room Chromecast',
    is_active: true,
    last_media_url: 'https://media.example.com/sample.mp4'
  },
  {
    device_name: 'Bedroom Chromecast',
    is_active: false
  }
];

export async function runClient(input: ChromecastInput): Promise<ChromecastOutput> {
  if (input.action === 'discover_devices' || input.action === 'status') {
    return {
      provider: 'chromecast',
      action: input.action,
      devices: BASE_DEVICES,
      summary: `Chromecast inventory contains ${BASE_DEVICES.length} devices.`
    };
  }

  return {
    provider: 'chromecast',
    action: input.action,
    devices: [
      {
        device_name: input.device_name ?? 'Living Room Chromecast',
        is_active: input.action !== 'stop',
        last_media_url: input.media_url
      }
    ],
    summary: `Chromecast action ${input.action} executed for ${input.device_name}.`
  };
}
