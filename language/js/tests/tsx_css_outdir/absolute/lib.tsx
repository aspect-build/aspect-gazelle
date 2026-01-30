// Test JSX img src with absolute paths (resolved relative to package.json directory, or workspace root as fallback)

export const AbsoluteImages = () => (
  <div>
    {/* Absolute path to root-level image */}
    <img src="/images/logo.webp" />
    {/* Absolute path to root-level image with query param */}
    <img src="/logo.png?v=1" />
    {/* Absolute path to sibling directory's image */}
    <img src="/sub/images/icon.gif" />
  </div>
)
