import type {
  ReactNode
} from "react";

import {
  Collection
} from "@trussworks/react-uswds";

import
  AvatarImage
from "../icon";

import "./notificationsList.scss";

export default function NotificationList(props: NotificationList.Props): ReactNode {

  const {
    notifications
  } = props;

  return (
    <Collection className="app-notificationList usa-collection--condensed margin-y-0">
      {notifications.map((notification) => (
        <li
          key={notification.id}
          className="usa-collection__item app-notificationList__item"
        >
          <div className="app-notificationList__image" aria-hidden="true">
            {AvatarImage(notification.image)}
          </div>

          <div className="usa-collection__body">
            <h4 className="usa-collection__heading app-notificationList__title">
              {notification.title}
            </h4>

            <p className="usa-collection__description app-notificationList__description">
              {notification.description}
            </p>
          </div>
        </li>
      ))}
    </Collection>
  );
}

export namespace NotificationList {

  export
  type Item = {
    id: string;
    image: string;
    title: string;
    description: string;
  };

  export
  type Props = {
    notifications: Item[];
  };
}
