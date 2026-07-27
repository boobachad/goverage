import React, { useState, useMemo } from 'react';
import { useStore } from '../store/useStore';

interface AuditLog {
  id?: number;
  timestamp: string;
  level: string;
  message: string;
  agent_id?: string;
  endpoint?: string;
  latency_ms?: number;
  tokens_used?: number;
  request_payload?: any;
  response_payload?: any;
}

interface SdkAuditEntry {
  timestamp: string;
  agent_id: string;
  action: string;
  decision: string;
  hash: string;
  previous_hash: string;
}

export const TelemetryTable: React.FC = () => {
  const telemetryBuffer = useStore((state) => state.telemetryBuffer);
  const version = useStore((state) => state.telemetryVersion);
  const auditLogs = useStore((state) => state.auditLogs);
  const fetchAuditLogs = useStore((state) => state.fetchAuditLogs);

  const [selectedLog, setSelectedLog] = useState<AuditLog | null>(null);
  const [verifyStatus, setVerifyStatus] = useState<'idle' | 'verifying' | 'intact' | 'tampered'>('idle');
  const [verifyData, setVerifyData] = useState<{intact: boolean, entries: SdkAuditEntry[]} | null>(null);

  const verifyLedger = async () => {
    setVerifyStatus('verifying');
    try {
      const res = await fetch('http://localhost:3000/api/system/audit/verify');
      if (res.ok) {
        const data = await res.json();
        setVerifyData(data);
        setVerifyStatus(data.intact ? 'intact' : 'tampered');
      } else {
        setVerifyStatus('tampered');
      }
    } catch (err) {
      console.error(err);
      setVerifyStatus('tampered');
    }
  };

  const telemetry = useMemo(() => {
    return [...telemetryBuffer.getItems(), ...auditLogs];
  }, [telemetryBuffer, version, auditLogs]);

  return (
    <div className="flex flex-col h-full w-full">
      <div className="flex justify-between items-center mb-2">
        <h2 className="text-lg font-bold uppercase tracking-wider">Audit & Telemetry Ledger</h2>
        <div className="flex gap-4">
          <button 
            onClick={verifyLedger}
            disabled={verifyStatus === 'verifying'}
            className="bg-blue-600 hover:bg-blue-700 text-white font-bold py-1 px-4 text-xs uppercase border border-black disabled:opacity-50"
          >
            Verify Hash Chain
          </button>
          <button 
            onClick={() => {
              fetchAuditLogs();
            }}
            className="bg-black hover:bg-gray-800 text-white font-bold py-1 px-4 text-sm uppercase border border-black"
          >
            Refresh History
          </button>
        </div>
      </div>

      {verifyStatus !== 'idle' && (
        <div className="mb-4 flex flex-col border border-black">
          <div className={`p-3 text-sm font-bold uppercase text-center border-b border-black ${
            verifyStatus === 'verifying' ? 'bg-yellow-200 text-yellow-800' :
            verifyStatus === 'intact' ? 'bg-green-500 text-white' :
            'bg-red-600 text-white animate-pulse'
          }`}>
            {verifyStatus === 'verifying' && 'Verifying Cryptographic Hash Chain...'}
            {verifyStatus === 'intact' && `CRYPTOGRAPHICALLY VERIFIED: Hash Chain Intact (${verifyData?.entries?.length || 0} Entries)`}
            {verifyStatus === 'tampered' && '[CAUTION] LEDGER TAMPERING DETECTED! HASH CHAIN BROKEN!'}
          </div>
          
          {verifyData?.entries && verifyData.entries.length > 0 && (
            <div className="bg-gray-50 overflow-x-auto p-4 flex gap-4">
              {verifyData.entries.map((entry, idx) => {
                const isBroken = verifyStatus === 'tampered' && idx === verifyData.entries.length - 1;
                return (
                  <div key={entry.hash} className={`flex-shrink-0 w-64 border p-3 flex flex-col gap-2 ${isBroken ? 'bg-red-100 border-red-500' : 'bg-white border-black'}`}>
                    <div className="text-[10px] font-mono text-gray-500 break-all">
                      PREV: {entry.previous_hash || 'GENESIS'}
                    </div>
                    <div>
                      <div className="text-xs font-bold uppercase">{entry.agent_id}</div>
                      <div className="text-sm">{entry.action}</div>
                      <div className={`text-xs font-bold ${entry.decision === 'allow' ? 'text-green-600' : 'text-red-600'}`}>
                        {entry.decision.toUpperCase()}
                      </div>
                    </div>
                    <div className="text-[10px] font-mono font-bold break-all mt-auto pt-2 border-t border-gray-200">
                      HASH: {entry.hash}
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      )}
      
      <div className="flex flex-1 overflow-hidden border border-black bg-white">
        <div className="w-1/2 border-r border-black overflow-y-auto p-4 bg-gray-50">
          {telemetry.map((log: any, idx) => {
            const isSelected = selectedLog === log;
            const status = log.status || log.level || 'info';
            return (
              <div 
                key={`${log.id || 'live'}-${idx}`} 
                onClick={() => setSelectedLog(log)}
                className={`cursor-pointer border border-black p-4 mb-4 ${isSelected ? 'bg-gray-200' : 'bg-white'}`}
              >
                <div className="flex justify-between items-center mb-2">
                  <span className="font-mono font-bold text-sm truncate">
                    {log.id ? `ID: #${log.id}` : 'WS Event'}
                  </span>
                  <span className={`px-2 py-0.5 text-xs font-bold uppercase border ${
                    status === 'success' || status === 'info' ? 'bg-black text-white border-black' : 
                    status === 'error' ? 'bg-white text-black border-black' : 'bg-gray-200 text-black border-black'
                  }`}>
                    {status}
                  </span>
                </div>
                <div className="text-sm font-mono mb-2 truncate">
                  {(() => {
                    const l = log;
                    if (l.message) return l.message;
                    if (l.response_payload?.choices?.[0]?.message) {
                      const m = l.response_payload.choices[0].message;
                      if (m.tool_calls?.length > 0) return `TOOL: ${m.tool_calls[0].function?.name}`;
                      if (m.content) return m.content;
                    }
                    return l.agent_id || 'System Event';
                  })()}
                </div>
                <div className="text-xs text-gray-500">
                  {log.timestamp ? new Date(log.timestamp).toLocaleTimeString() : '-'}
                </div>
              </div>
            );
          })}
          {telemetry.length === 0 && (
            <div className="text-center text-gray-500 p-4 font-bold uppercase">No logs found</div>
          )}
        </div>
        
        <div className="w-1/2 p-4 overflow-y-auto bg-white">
          {selectedLog ? (
            <div className="flex flex-col gap-4">
              <h3 className="text-lg font-bold uppercase tracking-wider border-b border-black pb-2">
                Log Detail: {selectedLog.id ? `#${selectedLog.id}` : 'WS Event'}
              </h3>
              
              {selectedLog.message && (
                <div>
                  <h4 className="text-xs font-bold uppercase mb-2 border-b border-black">Event Message</h4>
                  <pre className="bg-gray-100 text-black p-3 text-xs overflow-x-auto whitespace-pre-wrap font-mono border border-black">
                    {selectedLog.message}
                  </pre>
                </div>
              )}

              {selectedLog.request_payload && (
                <div>
                  <h4 className="text-xs font-bold uppercase mb-2 border-b border-black">Request Payload</h4>
                  <pre className="bg-black text-white p-3 text-xs overflow-x-auto whitespace-pre-wrap font-mono">
                    {JSON.stringify(selectedLog.request_payload, null, 2)}
                  </pre>
                </div>
              )}
              
              {selectedLog.response_payload && selectedLog.response_payload.choices?.[0]?.message && (
                <div>
                  <h4 className="text-xs font-bold uppercase mb-2 border-b border-black text-indigo-600">Extracted LLM Output</h4>
                  {selectedLog.response_payload.choices[0].message.content && (
                    <div className="bg-indigo-50 border border-indigo-200 text-indigo-900 p-3 text-sm mb-2 whitespace-pre-wrap font-mono">
                      {selectedLog.response_payload.choices[0].message.content}
                    </div>
                  )}
                  {selectedLog.response_payload.choices[0].message.tool_calls?.map((tc: any, i: number) => (
                    <div key={i} className="bg-amber-50 border border-amber-200 p-3 text-sm mb-2">
                      <div className="font-bold text-amber-900 mb-1 border-b border-amber-200 pb-1">TOOL INVOCATION: {tc.function?.name}</div>
                      <pre className="font-mono text-amber-800 text-xs overflow-x-auto">
                        {tc.function?.arguments}
                      </pre>
                    </div>
                  ))}
                </div>
              )}

              {selectedLog.response_payload && (
                <div>
                  <h4 className="text-xs font-bold uppercase mb-2 border-b border-black">Response Payload</h4>
                  <pre className="bg-black text-white p-3 text-xs overflow-x-auto whitespace-pre-wrap font-mono">
                    {JSON.stringify(selectedLog.response_payload, null, 2)}
                  </pre>
                </div>
              )}
            </div>
          ) : (
            <div className="h-full flex items-center justify-center text-gray-400 font-bold uppercase text-center p-4">
              Select a log to view details
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
