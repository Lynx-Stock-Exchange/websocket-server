package ws

func (c *Client) writePump() {
	defer func() {
		_ = c.conn.Close()
	}()

	for envelope := range c.send {
		if err := c.conn.WriteJSON(envelope); err != nil {
			return
		}
	}
}
