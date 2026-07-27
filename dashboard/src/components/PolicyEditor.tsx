import React from 'react';
import { useStore } from '../store/useStore';

export const PolicyEditor: React.FC = () => {
  const policyContent = useStore((state) => state.policyContent);
  const setPolicyContent = useStore((state) => state.setPolicyContent);

  return (
    <div className="flex-1 flex flex-col w-full">
      <div className="flex justify-between items-center mb-2">
        <h2 className="text-lg font-bold uppercase tracking-wider">Policy Editor</h2>
        <button
          onClick={async () => {
            try {
              await fetch('http://localhost:3000/api/policy', {
                method: 'POST',
                body: policyContent,
              });
            } catch (err) {
              console.error(err);
            }
          }}
          className="px-4 py-2 bg-black text-white hover:bg-gray-800 font-bold uppercase text-sm border border-black"
        >
          Save
        </button>
      </div>
      <textarea
        value={policyContent}
        onChange={(e) => setPolicyContent(e.target.value)}
        className="flex-1 w-full h-full p-4 font-mono text-sm border border-black focus:outline-none focus:ring-0 resize-none bg-gray-50 text-black"
        spellCheck={false}
      />
    </div>
  );
};
