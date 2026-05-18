export interface ParcelPackageTrackingInput {
  tracking_number: string;
  carrier?: 'auto' | 'ups' | 'usps' | 'fedex' | 'dhl';
  locale?: string;
}

export interface ParcelHistoryEvent {
  timestamp: string;
  location: string;
  description: string;
}

export interface ParcelPackageTrackingOutput {
  provider: 'parcel';
  tracking_number: string;
  carrier: 'ups' | 'usps' | 'fedex' | 'dhl';
  status: 'label_created' | 'in_transit' | 'out_for_delivery' | 'delivered';
  eta?: string;
  history: ParcelHistoryEvent[];
}
