import React, { useState } from 'react';
import { useStore } from '../store/useStore';

export const ActiveAgentsTable: React.FC = () => {
  const agentStatuses = useStore((state) => state.agentStatuses);
  const fleetBudget = useStore((state) => state.fleetBudget);
  const fetchAgentStatuses = useStore((state) => state.fetchAgentStatuses);

  // Inline editing state for Fleet
  const [editingFleet, setEditingFleet] = useState(false);
  const [fleetInput, setFleetInput] = useState('');

  // Inline editing state for individual Agents
  const [editingAgent, setEditingAgent] = useState<string | null>(null);
  const [agentInput, setAgentInput] = useState('');

  const handleSaveFleet = async () => {
    try {
      await fetch('http://localhost:3000/api/budget', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ target: 'fleet', amount: Number(fleetInput) }),
      });
      setEditingFleet(false);
      fetchAgentStatuses();
    } catch (err) {
      console.error(err);
    }
  };

  const handleSaveAgent = async (did: string) => {
    try {
      await fetch('http://localhost:3000/api/budget', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ target: did, amount: Number(agentInput) }),
      });
      setEditingAgent(null);
      fetchAgentStatuses();
    } catch (err) {
      console.error(err);
    }
  };

  return (
    <div className="flex flex-col p-4 bg-white gap-4 h-full">
      <div className="flex justify-between items-center">
        <div className="flex flex-col gap-1">
          <h2 className="text-lg font-bold uppercase tracking-wider">Active Agents</h2>
          <div className="flex items-center gap-2">
            <span className="text-sm text-gray-500 font-medium">Total Fleet Budget:</span>
            {editingFleet ? (
              <div className="flex items-center gap-1">
                <input
                  type="number"
                  value={fleetInput}
                  onChange={(e) => setFleetInput(e.target.value)}
                  className="border border-black px-2 py-0.5 text-sm w-24 focus:outline-none"
                  autoFocus
                />
                <button onClick={handleSaveFleet} className="text-xs bg-black text-white px-2 py-1 font-bold uppercase">Save</button>
                <button onClick={() => setEditingFleet(false)} className="text-xs border border-black bg-white text-black px-2 py-1 font-bold uppercase">Cancel</button>
              </div>
            ) : (
              <div className="flex items-center gap-2">
                <span className="text-sm text-gray-700 font-bold">{fleetBudget} tokens</span>
                <button 
                  onClick={() => { setFleetInput(String(fleetBudget)); setEditingFleet(true); }}
                  className="text-xs text-black border-b border-black font-bold uppercase hover:bg-gray-200"
                >
                  Edit
                </button>
              </div>
            )}
          </div>
        </div>
        <button
          onClick={fetchAgentStatuses}
          className="bg-black hover:bg-gray-800 text-white font-bold py-1 px-4 text-sm uppercase border border-black"
        >
          Refresh
        </button>
      </div>
      <div className="overflow-auto border-t border-b border-black bg-white">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-100 sticky top-0">
            <tr>
              <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">DID</th>
              <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th>
              <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Cap</th>
              <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Used</th>
              <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Remaining</th>
              <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Actions</th>
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200 text-sm">
            {agentStatuses.map((agent, index) => (
              <tr key={index} className="hover:bg-gray-50">
                <td className="px-4 py-2 whitespace-nowrap text-gray-900 font-mono">{agent.did}</td>
                <td className="px-4 py-2 whitespace-nowrap">
                  <span className={`px-2 inline-flex text-xs leading-5 font-bold uppercase ${
                    agent.status === 'active' ? 'bg-black text-white' :
                    agent.status === 'provisioning' ? 'bg-blue-600 text-white' :
                    agent.status === 'suspended' ? 'bg-yellow-400 text-black' :
                    agent.status === 'degraded' ? 'bg-orange-500 text-white' :
                    agent.status === 'quarantined' ? 'bg-red-600 text-white' :
                    agent.status === 'terminated' ? 'bg-gray-200 text-black' :
                    agent.status === 'halted' ? 'bg-red-700 text-white' :
                    'bg-gray-600 text-white'
                  }`}>
                    {agent.status}
                  </span>
                </td>
                <td className="px-4 py-2 whitespace-nowrap text-gray-500">
                  {editingAgent === agent.did ? (
                    <div className="flex items-center gap-1">
                      <input
                        type="number"
                        value={agentInput}
                        onChange={(e) => setAgentInput(e.target.value)}
                        className="border border-black px-2 py-0.5 text-sm w-20 focus:outline-none"
                        autoFocus
                      />
                      <button onClick={() => handleSaveAgent(agent.did)} className="text-xs bg-black text-white px-2 py-1 font-bold uppercase">Save</button>
                      <button onClick={() => setEditingAgent(null)} className="text-xs border border-black bg-white text-black px-2 py-1 font-bold uppercase">Cancel</button>
                    </div>
                  ) : (
                    <div className="flex items-center gap-2">
                      <span>{agent.cap}</span>
                      <button 
                        onClick={() => { setAgentInput(String(agent.cap)); setEditingAgent(agent.did); }}
                        className="text-xs text-black border-b border-black font-bold uppercase hover:bg-gray-200"
                      >
                        Edit
                      </button>
                    </div>
                  )}
                </td>
                <td className="px-4 py-2 whitespace-nowrap text-gray-500">{agent.used}</td>
                <td className="px-4 py-2 whitespace-nowrap text-gray-500">{agent.remaining}</td>
                <td className="px-4 py-2 whitespace-nowrap">
                  {agent.status === 'terminated' || agent.status === 'decommissioned' ? (
                    <span className="text-xs text-gray-400 italic">No actions available</span>
                  ) : (
                    <div className="flex flex-col gap-1">
                    {agent.status === 'halted' ? (
                      <button 
                        onClick={async () => {
                          try {
                            await fetch('http://localhost:3000/api/resume', {
                              method: 'POST',
                              headers: { 'Content-Type': 'application/json' },
                              body: JSON.stringify({ agent_id: agent.did }),
                            });
                            fetchAgentStatuses();
                          } catch (err) {
                            console.error(err);
                          }
                        }}
                        className="text-xs bg-green-600 text-white px-3 py-1 font-bold uppercase hover:bg-green-700"
                      >
                        Lift Halt
                      </button>
                    ) : (
                      <button 
                        onClick={async () => {
                          try {
                            await fetch('http://localhost:3000/api/halt', {
                              method: 'POST',
                              headers: { 'Content-Type': 'application/json' },
                              body: JSON.stringify({ agent_id: agent.did }),
                            });
                            fetchAgentStatuses();
                          } catch (err) {
                            console.error(err);
                          }
                        }}
                        className="text-xs bg-black text-white px-3 py-1 font-bold uppercase hover:bg-gray-800 disabled:opacity-50 disabled:cursor-not-allowed"
                        disabled={agent.status !== 'active'}
                      >
                        Halt
                      </button>
                    )}

                    {(agent.status === 'suspended' || agent.status === 'quarantined') && (
                      <button 
                        onClick={async () => {
                          try {
                            await fetch('http://localhost:3000/api/lifecycle/resume', {
                              method: 'POST',
                              headers: { 'Content-Type': 'application/json' },
                              body: JSON.stringify({ agent_id: agent.did }),
                            });
                            fetchAgentStatuses();
                          } catch (err) {
                            console.error(err);
                          }
                        }}
                        className="text-xs bg-blue-600 text-white px-3 py-1 font-bold uppercase hover:bg-blue-700"
                      >
                        Activate
                      </button>
                    )}

                    {agent.status === 'active' && (
                      <>
                        <button 
                          onClick={async () => {
                            try {
                              await fetch('http://localhost:3000/api/lifecycle/suspend', {
                                method: 'POST',
                                headers: { 'Content-Type': 'application/json' },
                                body: JSON.stringify({ agent_id: agent.did }),
                              });
                              fetchAgentStatuses();
                            } catch (err) {
                              console.error(err);
                            }
                          }}
                          className="text-xs bg-yellow-500 text-black px-3 py-1 font-bold uppercase hover:bg-yellow-600"
                        >
                          Suspend
                        </button>
                        <button 
                          onClick={async () => {
                            try {
                              await fetch('http://localhost:3000/api/lifecycle/quarantine', {
                                method: 'POST',
                                headers: { 'Content-Type': 'application/json' },
                                body: JSON.stringify({ agent_id: agent.did }),
                              });
                              fetchAgentStatuses();
                            } catch (err) {
                              console.error(err);
                            }
                          }}
                          className="text-xs bg-red-600 text-white px-3 py-1 font-bold uppercase hover:bg-red-700"
                        >
                          Quarantine
                        </button>
                      </>
                    )}
                  </div>
                  )}
                </td>
              </tr>
            ))}
            {agentStatuses.length === 0 && (
              <tr>
                <td colSpan={6} className="px-4 py-4 text-center text-gray-500">No agents found</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
};
