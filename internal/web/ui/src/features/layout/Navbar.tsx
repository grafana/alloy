import { NavLink } from 'react-router-dom';

import logo from '../../images/logo.svg';

import styles from './Navbar.module.css';

function Navbar() {
  return (
    <nav className={styles.navbar}>
      <header>
        <NavLink to="/">
          <img src={logo} alt="Grafana Alloy Logo" title="Grafana Alloy" />
        </NavLink>
      </header>
      <ul>
        <li>
          {/* Use a regular <a> tag to reload the page when the link is clicked */}
          <a href="/graph" className="nav-link">
            Graph
          </a>
        </li>
        <li>
          <NavLink to="/clustering" className="nav-link">
            Clustering
          </NavLink>
        </li>
        <li>
          <NavLink to="/remotecfg" className="nav-link">
            Remote Configuration
          </NavLink>
        </li>
        <li>
          <a href="https://grafana.com/docs/alloy/latest">Help</a>
        </li>
      </ul>
    </nav>
  );
}

export default Navbar;
