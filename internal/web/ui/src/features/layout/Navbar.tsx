import { NavLink } from 'react-router';

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
          <NavLink to="/graph" className="nav-link">
            Graph
          </NavLink>
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
          <a href="/-/support">Support Bundle</a>
        </li>
        <li>
          <a href="https://grafana.com/docs/alloy/latest">Help</a>
        </li>
      </ul>
    </nav>
  );
}

export default Navbar;
