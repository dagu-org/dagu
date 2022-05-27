import React from "react";
import Logo from "./Logo";

type Props = {
  title: string;
};

function Nav({ title }: Props) {
  return (
    <nav
      id="nav"
      className="navbar"
      role="navigation"
      aria-label="main navigation"
    >
      <div className="navbar-brand">
        <a className="navbar-item" href="/">
          <Logo />
          <span id="nav-title">{title}</span>
        </a>
      </div>
    </nav>
  );
}
export default Nav;
