import React from 'react';

export const ExecutionHaltButton: React.FC = () => {

  const handleStop = async () => {
    try {
      const response = await fetch('http://localhost:3000/api/halt', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({ agent_id: 'fleet' }),
      });
      if (!response.ok) {
        console.error('Failed to trigger execution halt');
      }
    } catch (error) {
      console.error('Error triggering execution halt:', error);
    }
  };

  const handleResume = async () => {
    try {
      const response = await fetch('http://localhost:3000/api/resume', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({ agent_id: 'fleet' }),
      });
      if (!response.ok) {
        console.error('Failed to trigger execution resume');
      }
    } catch (error) {
      console.error('Error triggering execution resume:', error);
    }
  };

  return (
    <div className="flex items-center gap-2">
      <button
        onClick={handleResume}
        className="bg-green-600 hover:bg-green-700 text-white font-bold h-10 px-4 active:bg-green-800 border border-black uppercase text-sm"
      >
        Lift Global Halt
      </button>
      <button
        onClick={handleStop}
        className="bg-black hover:bg-gray-800 text-white font-bold h-10 px-4 active:bg-gray-600 border border-black uppercase text-sm"
      >
        Full Execution Halt
      </button>
    </div>
  );
};
