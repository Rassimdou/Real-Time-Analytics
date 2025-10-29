"use client"
import { useState } from "react";
import {
    Plus,
    LayoutDashboard,
    LineChart,
    Settings,
    Folder,
    Bell,
    HelpCircle,
    icons
} from "lucide-react";

export function Sidebar() {
    const [active , setActive] = useState("Dashboard");

    const menuItems = [
    { name: "Dashboard",icon: <LayoutDashboard  size={15}/>,path: "/client/dashboard" },
    { name: "Analytics",icon: <LineChart size={15}/>, path: "/client/analytics" },
    { name: "Reports",icon: <Folder size={15}/>,path: "/client/reports" },
    { name: "Settings",icon: <Settings size={15}/>,path: "/client/settings" },
    { name: "Alerts",icon : <Bell size={15}/>,path: "/client/alerts" },
    { name: "Help",icon: <HelpCircle size={15}/>,path: "/client/help" },
    ]


    return (
        <aside className="w-60 h-screen bg-gray-300 shadow-sm text-gray-700 px-4 py-5 border-r border-gray-200">
            <h1 className="text-lg mb-6 font-semibold">Client Panel</h1>
            <ul className="space-y-1.5">
                {menuItems.map((item) =>(
                <li key={item.name}>
                    <a href={item.path}
                    onClick={()=> setActive(item.name)}
                    className={`flex items-center gap-3 px-3 py-2 rounded-md transition-all duration-200
                    ${active ===item.name ? 
                        "bg-blue-300 text-gray-800 font-medium" :
                        "texy-gray-500 hover:bg-purple-100 hover:text-gray-900"
                    }`}
                
                    >
                      {item.icon}{item.name}
                        
                    </a> 
                </li>
                ))}
            </ul>   
        </aside>
    
    );
}