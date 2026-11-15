import { useState, useEffect } from 'react';

interface MessageResponse {
  message: string;
}

const App = () => {
  const [data, setData] = useState<string>('');
  const [loading, setLoading] = useState<boolean>(true);

  useEffect(() => {
    fetch('http://localhost:8080/api/health')
      .then((res) => {
        if (!res.ok) throw new Error('Network response was not ok');
        return res.json() as Promise<MessageResponse>;
      })
      .then((data) => {
        setData(data.message);
        setLoading(false);
      })
      .catch((err) => {
        console.error("Error fetching data:", err);
        setLoading(false);
      });
  }, []);

  return (
    <div style={{ textAlign: 'center', marginTop: '50px', fontFamily: 'sans-serif' }}>
      <h1>Go + React (TypeScript)</h1>
      {loading ? (
        <p>Loading API data...</p>
      ) : (
        <p>Backend Response: <strong>{data}</strong></p>
      )}
    </div>
  );
}

export default App;
