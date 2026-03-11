import type {
  ReactNode
} from "react";

import {
  Card,
  CardBody,
  CardFooter,
  Button,
  Tag
} from "@trussworks/react-uswds";

import {
  StatusTag
} from "../StatusTag"

import AvatarImage from "../icon";

import pinIconUrl from "../../assets/pin.svg";
import unpinIconUrl from "../../assets/unpin.svg";

import "./index.scss";

export function DetailedCard(props: DetailedCardProps): ReactNode {
  // Expand the props
  const {
    image,
    status,
    name,
    description,
    category,
    pinned,
    url,
    onTogglePin,
  } = props;

  return (
    <Card className="app-card app-card--detailed" layout="standardDefault">
      <a
        href={url}
        target="_blank"
        rel="noopener noreferrer"
        className="app-card-detailed__mainLink"
        aria-label={`${name} (opens in a new tab)`}
      >
        <CardBody className="app-card__body app-card__body--detailed">
          <div className="app-card-detailed__topRow">
            <div className="app-card-detailed__avatar">
              {AvatarImage(image)}
            </div>

            <StatusTag status={status} />
          </div>

          <h3 className="app-card__title app-card__title--detailed">{name}</h3>

          <div className="app-card-detailed__description usa-prose">{description}</div>
        </CardBody>
      </a>

      <CardFooter className="app-card__footer app-card__footer--detailed">
        <div className="app-card-detailed__footerRow">
          <div className="app-card-detailed__categories">
            {category.map((item) => (
              <Tag
                key={item}
                background="base-lighter"
                className="app-card-detailed__category"
              >
                {item}
              </Tag>
            ))}
          </div>

          <Button
            type="button"
            unstyled
            className="app-card-detailed__pinButton"
            onClick={() => onTogglePin(!pinned)}
            title={pinned ? "Unpin" : "Pin"}
          >
            <img
              src={pinned ? unpinIconUrl : pinIconUrl}
              className="app-card-detailed__pinIcon"
              alt=""
            />
          </Button>
        </div>
      </CardFooter>
    </Card>
  );
}

export type DetailedCardProps = {
  className?: string;
  image: string;
  status: string;
  name: string;
  description: string;
  category: string[];
  pinned: boolean;
  url: string;
  onTogglePin: (nextPinned: boolean) => void;
};
