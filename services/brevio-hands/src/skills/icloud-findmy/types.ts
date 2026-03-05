export interface FindMyInput {
  device_name?: string;
}

export interface FindMyDevice {
  name: string;
  latitude: number;
  longitude: number;
  battery: number;
}

export interface FindMyOutput {
  provider: 'icloud-findmy';
  devices: FindMyDevice[];
}
