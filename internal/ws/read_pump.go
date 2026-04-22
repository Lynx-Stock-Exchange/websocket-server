package ws

func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c)
		_ = c.conn.Close()
	}()

	for {
		var envelope IncomingEnvelope
		if err := c.conn.ReadJSON(&envelope); err != nil {
			return
		}

		// TODO: Route SUBSCRIBE to subscription handling.
		// TODO: Route PLACE_ORDER to order handling.
	}
}
