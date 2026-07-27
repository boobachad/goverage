import { create } from 'zustand';

export interface TelemetryEvent {
  timestamp?: string;
  level?: string;
  message?: string;
  [key: string]: any;
}

const MAX_TELEMETRY_COUNT = 1000;

class RingBuffer<T> {
  buffer: T[];
  head: number;
  size: number;
  capacity: number;

  constructor(capacity: number) {
    this.capacity = capacity;
    this.buffer = new Array(capacity);
    this.head = 0;
    this.size = 0;
  }

  push(item: T) {
    this.buffer[this.head] = item;
    this.head = (this.head + 1) % this.capacity;
    if (this.size < this.capacity) {
      this.size++;
    }
  }

  // Returns items from newest to oldest
  getItems(): T[] {
    const items = new Array(this.size);
    for (let i = 0; i < this.size; i++) {
      const index = (this.head - 1 - i + this.capacity) % this.capacity;
      items[i] = this.buffer[index];
    }
    return items;
  }
}

interface AppState {
  telemetryBuffer: RingBuffer<TelemetryEvent>;
  // We keep a version counter to trigger React re-renders efficiently without copying the buffer in state every time
  telemetryVersion: number;
  addTelemetryEvent: (event: TelemetryEvent) => void;
  policyContent: string;
  setPolicyContent: (content: string) => void;
  auditLogs: TelemetryEvent[];
  fetchAuditLogs: () => Promise<void>;
  agentStatuses: any[];
  fleetBudget: number;
  fetchAgentStatuses: () => Promise<void>;
  fetchPolicy: () => Promise<void>;
}

export const useStore = create<AppState>((set) => ({
  telemetryBuffer: new RingBuffer<TelemetryEvent>(MAX_TELEMETRY_COUNT),
  telemetryVersion: 0,
  addTelemetryEvent: (event) => set((state) => {
    state.telemetryBuffer.push(event);
    return { telemetryVersion: state.telemetryVersion + 1 };
  }),
  policyContent: '',
  setPolicyContent: (content) => set({ policyContent: content }),
  auditLogs: [],
  fetchAuditLogs: async () => {
    try {
      const res = await fetch('http://localhost:3000/api/audit');
      if (res.ok) {
        const data = await res.json();
        set({ auditLogs: Array.isArray(data) ? data : [] });
      } else {
        console.error('Failed to fetch audit logs');
      }
    } catch (err) {
      console.error('Error fetching audit logs:', err);
    }
  },
  agentStatuses: [],
  fleetBudget: 0,
  fetchAgentStatuses: async () => {
    try {
      const res = await fetch('http://localhost:3000/api/agents/status');
      if (res.ok) {
        const data = await res.json();
        set({ 
          agentStatuses: Array.isArray(data.agents) ? data.agents : [],
          fleetBudget: typeof data.fleet_budget === 'number' ? data.fleet_budget : 0
        });
      } else {
        console.error('Failed to fetch agent statuses');
      }
    } catch (err) {
      console.error('Error fetching agent statuses:', err);
    }
  },
  fetchPolicy: async () => {
    try {
      const res = await fetch('http://localhost:3000/api/policy');
      if (res.ok) {
        const text = await res.text();
        set({ policyContent: text });
      } else {
        console.error('Failed to fetch policy');
      }
    } catch (err) {
      console.error('Error fetching policy:', err);
    }
  }
}));
