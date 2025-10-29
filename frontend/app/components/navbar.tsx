export default function Navbar() {
  return (
    <nav className="fixed top-0 left-0 w-full z-50 bg-transparent shadow-none">
      <div className="flex items-center justify-between px-8 py-4 text-white">
        {/* Logo */}
        <div className="text-2xl font-bold">RTealytics</div>

        {/* Links */}
        <ul className="flex space-x-8 text-lg">
          <li className="cursor-pointer hover:text-gray-300">Home</li>
          <li className="cursor-pointer hover:text-gray-300">Our Services</li>
          <li className="cursor-pointer hover:text-gray-300">Pricing</li>
        </ul>

        {/* Auth buttons */}
        <div>
          <button className="px-4 py-2 border border-white rounded hover:bg-white hover:text-black transition">
            Login
          </button>
        </div>
      </div>
    </nav>
  );
}
