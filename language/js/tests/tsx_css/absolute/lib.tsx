// Test JSX img src with absolute paths (resolved relative to workspace root)

export const AbsoluteImages = () => (
  <div>
    {/* Absolute path to root-level image */}
    <img src="/images/logo.png" />
    {/* Absolute path to root-level image with query param */}
    <img src="/logo.png?v=1" />
    {/* Absolute path to sibling directory's image */}
    <img src="/sub/images/sub.svg" />
  </div>
)
