import React from 'react';

// Object for react-syntax-highlighter's custom theme
export const style: {
  [key: string]: React.CSSProperties;
} = {
  'code[class*="language-"]': {
    color: 'black',
    background: 'none',
    fontFamily: 'Fira Code,monospace',
    textAlign: 'left',
    whiteSpace: 'pre',
    wordSpacing: 'normal',
    wordBreak: 'normal',
    wordWrap: 'normal',
    MozTabSize: '4',
    OTabSize: '4',
    tabSize: '4',
    WebkitHyphens: 'none',
    MozHyphens: 'none',
    msHyphens: 'none',
    hyphens: 'none',
  },
  'pre[class*="language-"]': {
    background: 'none',
    fontFamily: 'Fira Code,monospace',
    textAlign: 'left',
    whiteSpace: 'pre',
    wordSpacing: 'normal',
    wordBreak: 'normal',
    wordWrap: 'normal',
    MozTabSize: '4',
    OTabSize: '4',
    tabSize: '4',
    WebkitHyphens: 'none',
    MozHyphens: 'none',
    msHyphens: 'none',
    hyphens: 'none',
    margin: '0.5em 0',
    overflowX: 'auto',
    overflowY: 'hidden',
    borderRadius: '0.3em',
  },
  ':not(pre) > code[class*="language-"]': {
    background: 'none',
    borderRadius: '0.3em',
    whiteSpace: 'normal',
  },
  comment: {
    color: '#d4d0ab',
  },
  prolog: {
    color: '#d4d0ab',
  },
  doctype: {
    color: '#d4d0ab',
  },
  cdata: {
    color: '#d4d0ab',
  },
  property: {
    color: '#ffa07a',
  },
  tag: {
    color: '#ffa07a',
  },
  constant: {
    color: '#ffa07a',
  },
  symbol: {
    color: '#ffa07a',
  },
  deleted: {
    color: '#ffa07a',
  },
  boolean: {
    color: 'blue',
  },
  number: {
    color: 'blue',
  },
  selector: {
    color: '#abe338',
  },
  'attr-name': {
    color: '#abe338',
  },
  string: {
    color: 'green',
  },
  char: {
    color: '#abe338',
  },
  builtin: {
    color: '#abe338',
  },
  inserted: {
    color: '#abe338',
  },
  entity: {
    color: '#00e0e0',
    cursor: 'help',
  },
  url: {
    color: '#00e0e0',
  },
  '.language-css .token.string': {
    color: '#00e0e0',
  },
  '.style .token.string': {
    color: '#00e0e0',
  },
  variable: {
    color: '#00e0e0',
  },
  atrule: {
    color: 'grey',
  },
  'attr-value': {
    color: 'grey',
  },
  function: {
    color: 'grey',
  },
  keyword: {
    color: '#00e0e0',
  },
  regex: {
    color: '#ffd700',
  },
  important: {
    color: '#ffd700',
    fontWeight: 'bold',
  },
  bold: {
    fontWeight: 'bold',
  },
  italic: {
    fontStyle: 'italic',
  },
};
