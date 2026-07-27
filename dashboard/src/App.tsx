import { useEffect, useState } from 'react'
import { useStore } from './store/useStore'
import { ExecutionHaltButton } from './components/ExecutionHaltButton'
import { PolicyEditor } from './components/PolicyEditor'
import { TelemetryTable } from './components/TelemetryTable'
import { ActiveAgentsTable } from './components/ActiveAgentsTable'

function App() {
  const [activeTab, setActiveTab] = useState<'agents' | 'policy' | 'telemetry'>('agents');
  const addTelemetryEvent = useStore((state) => state.addTelemetryEvent);
  const fetchPolicy = useStore((state) => state.fetchPolicy);
  const fetchAgentStatuses = useStore((state) => state.fetchAgentStatuses);

  useEffect(() => {
    fetchPolicy();
    fetchAgentStatuses();
    const ws = new WebSocket('ws://localhost:3000/ws/telemetry');

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        addTelemetryEvent(data);
      } catch (err) {
        // Fallback for non-JSON strings
        addTelemetryEvent({
          timestamp: new Date().toISOString(),
          level: 'info',
          message: event.data,
        });
      }
    };

    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    const interval = setInterval(() => fetchAgentStatuses(), 5000);

    return () => {
      ws.close();
      clearInterval(interval);
    };
  }, [addTelemetryEvent, fetchPolicy, fetchAgentStatuses]);

  return (
    <div className="min-h-screen bg-white text-black p-0 flex flex-col">
      <header className="flex justify-between items-center bg-gray-200 p-4 border-b border-black">
        <h1 className="text-xl font-bold uppercase tracking-wider">Goverage Dashboard</h1>
        <div className="flex gap-4 items-center">
          <ExecutionHaltButton />
        </div>
      </header>

      <div className="flex border-b border-black bg-gray-100">
        <button 
          onClick={() => setActiveTab('agents')} 
          className={`px-6 py-2 border-r border-black font-semibold uppercase text-sm ${activeTab === 'agents' ? 'bg-black text-white' : 'hover:bg-gray-300'}`}
        >
          Active Agents
        </button>
        <button 
          onClick={() => setActiveTab('policy')} 
          className={`px-6 py-2 border-r border-black font-semibold uppercase text-sm ${activeTab === 'policy' ? 'bg-black text-white' : 'hover:bg-gray-300'}`}
        >
          Policy Editor
        </button>
        <button 
          onClick={() => setActiveTab('telemetry')} 
          className={`px-6 py-2 border-r border-black font-semibold uppercase text-sm ${activeTab === 'telemetry' ? 'bg-black text-white' : 'hover:bg-gray-300'}`}
        >
          Telemetry
        </button>
      </div>

      <main className="flex-1 p-6 flex flex-col">
        {activeTab === 'agents' && (
          <div className="flex-1 flex flex-col border border-black">
            <ActiveAgentsTable />
          </div>
        )}
        
        {activeTab === 'policy' && (
          <div className="flex-1 flex flex-col border border-black p-4 min-h-[600px]">
            <PolicyEditor />
          </div>
        )}
        
        {activeTab === 'telemetry' && (
          <div className="flex-1 flex flex-col border border-black p-4">
            <TelemetryTable />
          </div>
        )}
      </main>
    </div>
  )
}

export default App
