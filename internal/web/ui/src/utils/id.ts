type ID = {
  moduleID: string;
  localID: string;
};

/**
 * parseID parses a full component ID into its moduleID and localID halves.
 */
export function parseID(id: string): ID {
  const lastSlashIndex = id.lastIndexOf('/');
  if (lastSlashIndex === -1) {
    return { moduleID: '', localID: id };
  }

  return {
    moduleID: id.slice(0, lastSlashIndex),
    localID: id.slice(lastSlashIndex + 1),
  };
}
