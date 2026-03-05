import type { FindMyDevice, FindMyInput, FindMyOutput } from './types.js';

const DEVICES: FindMyDevice[] = [
  {
    name: 'Albert MacBook Pro',
    latitude: 37.7765,
    longitude: -122.417,
    battery: 82
  },
  {
    name: 'iPhone 15 Pro',
    latitude: 37.7792,
    longitude: -122.4141,
    battery: 67
  },
  {
    name: 'AirPods Pro',
    latitude: 37.7751,
    longitude: -122.4193,
    battery: 54
  }
];

export async function runClient(input: FindMyInput): Promise<FindMyOutput> {
  const query = input.device_name?.toLowerCase();
  return {
    provider: 'icloud-findmy',
    devices: DEVICES.filter((device) => !query || device.name.toLowerCase().includes(query))
  };
}
