import type { GoogleMapsInput, GoogleMapsOutput, RouteStep, TravelMode } from './types.js';

function hashRoute(input: string): number {
  let hash = 2166136261;
  for (let i = 0; i < input.length; i += 1) {
    hash ^= input.charCodeAt(i);
    hash +=
      (hash << 1) +
      (hash << 4) +
      (hash << 7) +
      (hash << 8) +
      (hash << 24);
  }
  return hash >>> 0;
}

function modeSpeedMetersPerSecond(mode: TravelMode): number {
  switch (mode) {
    case 'walking':
      return 1.3;
    case 'bicycling':
      return 5.5;
    case 'transit':
      return 8.5;
    default:
      return 13.5;
  }
}

function splitSteps(distanceMeters: number): RouteStep[] {
  const leg = Math.max(100, Math.floor(distanceMeters / 3));
  return [
    {
      instruction: 'Head to primary route',
      distance_m: leg
    },
    {
      instruction: 'Continue on main segment',
      distance_m: leg
    },
    {
      instruction: 'Arrive at destination',
      distance_m: Math.max(100, distanceMeters - leg - leg)
    }
  ];
}

export async function runClient(input: GoogleMapsInput): Promise<GoogleMapsOutput> {
  const mode = input.mode ?? 'driving';
  const routeHash = hashRoute(`${input.origin}|${input.destination}|${mode}`);

  const baseDistance = 1200 + (routeHash % 45000);
  const speed = modeSpeedMetersPerSecond(mode);
  const duration = Math.max(120, Math.round(baseDistance / speed));

  return {
    distance_m: baseDistance,
    duration_s: duration,
    mode,
    steps: splitSteps(baseDistance)
  };
}
