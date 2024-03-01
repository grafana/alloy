import { NavLink } from 'react-router-dom';

import logo from '../../images/logo.svg';

import styles from './Navbar.module.css';

function Navbar() {
  return (
    <nav className={styles.navbar}>
      <header>
        <NavLink to="/">
          <img src={logo} alt="Grafana Agent Logo" title="Grafana Agent" />
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
          <a href="https://grafana.com/docs/agent/latest">Help</a>
        </li>
      </ul>
    </nav>
  );
}

export default Navbar;
