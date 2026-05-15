export default function access(initialState: { currentUser?: API.CurrentUser } | undefined) {
  return {
    canAdmin: !!initialState?.currentUser,
  };
}
