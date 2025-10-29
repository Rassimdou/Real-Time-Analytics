import Navbar from "./components/navbar";
import { Sidebar } from "./dashboard-client/sidebar";

export default function Home() {
  return (
    <main className="min-h-screen bg-gradient-to-b from-blue-900 to-indigo-800 text-white">
      <Sidebar />
      
    </main>
  );
}
